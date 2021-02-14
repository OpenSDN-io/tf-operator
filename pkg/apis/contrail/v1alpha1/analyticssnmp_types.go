package v1alpha1

import (
	"bytes"
	"context"
	"sort"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
)

// AnalyticsSnmp is the Schema for the Analytics SNMP API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=analyticssnmp,scope=Namespaced
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type AnalyticsSnmp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalyticsSnmpSpec   `json:"spec,omitempty"`
	Status AnalyticsSnmpStatus `json:"status,omitempty"`
}

// AnalyticsSnmpList contains a list of AnalyticsSnmp.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AnalyticsSnmpList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AnalyticsSnmp `json:"items"`
}

// AnalyticsSnmpSpec is the Spec for the Analytics SNMP API.
// +k8s:openapi-gen=true
type AnalyticsSnmpSpec struct {
	CommonConfiguration  PodConfiguration           `json:"commonConfiguration,omitempty"`
	ServiceConfiguration AnalyticsSnmpConfiguration `json:"serviceConfiguration"`
}

// AnalyticsSnmpConfiguration is the Spec for the Analytics SNMP API.
// +k8s:openapi-gen=true
type AnalyticsSnmpConfiguration struct {
	CassandraInstance                 string       `json:"cassandraInstance,omitempty"`
	ZookeeperInstance                 string       `json:"zookeeperInstance,omitempty"`
	RabbitmqInstance                  string       `json:"rabbitmqInstance,omitempty"`
	ConfigInstance                    string       `json:"configInstance,omitempty"`
	LogFilePath                       string       `json:"logFilePath,omitempty"`
	LogLevel                          string       `json:"logLevel,omitempty"`
	LogLocal                          string       `json:"logLocal,omitempty"`
	SnmpCollectorScanFrequency        *int         `json:"snmpCollectorScanFrequency,omitempty"`
	SnmpCollectorFastScanFrequency    *int         `json:"snmpCollectorFastScanFrequency,omitempty"`
	SnmpCollectorIntrospectListenPort *int         `json:"snmpCollectorIntrospectListenPort,omitempty"`
	SnmpCollectorLogFileName          string       `json:"snmpCollectorLogFileName,omitempty"`
	TopologyScanFrequency             *int         `json:"topologySnmpFrequency,omitempty"`
	TopologyIntrospectListenPort      *int         `json:"topologyIntrospectListenPort,omitempty"`
	TopologyLogFileName               string       `json:"topologyLogFileName,omitempty"`
	Containers                        []*Container `json:"containers,omitempty"`
}

// AnalyticsSnmpStatus is the Status for the Analytics SNMP API.
// +k8s:openapi-gen=true
type AnalyticsSnmpStatus struct {
	Active        *bool             `json:"active,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AnalyticsSnmp{}, &AnalyticsSnmpList{})
}

// CreateConfigMap creates analytics snmp config map
func (c *AnalyticsSnmp) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {

	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"analyticssnmp",
		c)
}

// InstanceConfiguration create config data
func (c *AnalyticsSnmp) InstanceConfiguration(configMapName string,
	podList []corev1.Pod,
	request reconcile.Request,
	client client.Client) error {

	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(c.Spec.ServiceConfiguration.CassandraInstance,
		request.Namespace, client)
	if err != nil {
		return err
	}
	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(c.Spec.ServiceConfiguration.ZookeeperInstance,
		request.Namespace, client)
	if err != nil {
		return err
	}
	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(c.Spec.ServiceConfiguration.RabbitmqInstance, request.Namespace, client)
	if err != nil {
		return err
	}
	configNodesInformation, err := NewConfigClusterConfiguration(c.Spec.ServiceConfiguration.ConfigInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	var rabbitmqSecretUser string
	var rabbitmqSecretPassword string
	var rabbitmqSecretVhost string
	if rabbitmqNodesInformation.Secret != "" {
		rabbitmqSecret := &corev1.Secret{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqNodesInformation.Secret, Namespace: request.Namespace}, rabbitmqSecret)
		if err != nil {
			return err
		}
		rabbitmqSecretUser = string(rabbitmqSecret.Data["user"])
		rabbitmqSecretPassword = string(rabbitmqSecret.Data["password"])
		rabbitmqSecretVhost = string(rabbitmqSecret.Data["vhost"])
	}

	// Create main common values
	rabbitMqSSLEndpointList := configtemplates.EndpointList(rabbitmqNodesInformation.ServerIPList, rabbitmqNodesInformation.Port)
	sort.Strings(rabbitMqSSLEndpointList)
	rabbitmqSSLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, " ")

	configDbEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
	sort.Strings(configDbEndpointList)
	configDbEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configDbEndpointList, " ")

	configCollectorEndpointList := configtemplates.EndpointList(configNodesInformation.CollectorServerIPList, configNodesInformation.CollectorPort)
	sort.Strings(configCollectorEndpointList)
	configCollectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configCollectorEndpointList, " ")

	configApiEndpointList := configtemplates.EndpointList(configNodesInformation.APIServerIPList, configNodesInformation.APIServerPort)
	sort.Strings(configApiEndpointList)
	configApiIPEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, " ")

	configApiList := make([]string, len(configNodesInformation.APIServerIPList))
	copy(configApiList, configNodesInformation.APIServerIPList)
	sort.Strings(configApiList)
	configApiIPCommaSeparated := configtemplates.JoinListWithSeparator(configApiList, ",")

	zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
	sort.Strings(zookeeperEndpointList)
	zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")

	var data = make(map[string]string)
	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		var collectorBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpCollectorConfig.Execute(&collectorBuffer, struct {
			PodIP                             string
			Hostname                          string
			ListenAddress                     string
			InstrospectListenAddress          string
			SnmpCollectorScanFrequency        string
			SnmpCollectorFastScanFrequency    string
			SnmpCollectorIntrospectListenPort string
			LogFile                           string
			LogLevel                          string
			LogLocal                          string
			CollectorServers                  string
			ZookeeperServers                  string
			ConfigServers                     string
			ConfigDbServerList                string
			CassandraSslCaCertfile            string
			RabbitmqServerList                string
			RabbitmqVhost                     string
			RabbitmqUser                      string
			RabbitmqPassword                  string
			CAFilePath                        string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServers:         configCollectorEndpointListSpaceSeparated,
			ZookeeperServers:         zookeeperEndpointListCommaSeparated,
			ConfigServers:            configApiIPEndpointListSpaceSeparated,
			ConfigDbServerList:       configDbEndpointListSpaceSeparated,
			CassandraSslCaCertfile:   certificates.SignerCAFilepath,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqVhost:            rabbitmqSecretVhost,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			CAFilePath:               certificates.SignerCAFilepath,
			// TODO: move to params
			LogLevel: "SYS_DEBUG",
		})
		data["tf-snmp-collector."+podIP] = collectorBuffer.String()

		var topologyBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpTopologyConfig.Execute(&topologyBuffer, struct {
			PodIP                            string
			Hostname                         string
			ListenAddress                    string
			InstrospectListenAddress         string
			SnmpTopologyScanFrequency        string
			SnmpTopologyIntrospectListenPort string
			LogFile                          string
			LogLevel                         string
			LogLocal                         string
			CollectorServers                 string
			ZookeeperServers                 string
			AnalyticsServers                 string
			ConfigServers                    string
			ConfigDbServerList               string
			CassandraSslCaCertfile           string
			RabbitmqServerList               string
			RabbitmqVhost                    string
			RabbitmqUser                     string
			RabbitmqPassword                 string
			CAFilePath                       string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServers:         configCollectorEndpointListSpaceSeparated,
			ZookeeperServers:         zookeeperEndpointListCommaSeparated,
			AnalyticsServers:         configApiIPEndpointListSpaceSeparated,
			ConfigServers:            configApiIPEndpointListSpaceSeparated,
			ConfigDbServerList:       configDbEndpointListSpaceSeparated,
			CassandraSslCaCertfile:   certificates.SignerCAFilepath,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqVhost:            rabbitmqSecretVhost,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			CAFilePath:               certificates.SignerCAFilepath,
			// TODO: move to params
			LogLevel: "SYS_DEBUG",
		})
		data["tf-topology."+podIP] = topologyBuffer.String()

		// TODO: commonize for all services
		var nodemanagerBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpNodemanagerConfig.Execute(&nodemanagerBuffer, struct {
			PodIP                    string
			Hostname                 string
			ListenAddress            string
			InstrospectListenAddress string
			LogFile                  string
			LogLevel                 string
			LogLocal                 string
			CassandraPort            string
			CassandraJmxPort         string
			CAFilePath               string
			CollectorServerList      string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               certificates.SignerCAFilepath,
			CollectorServerList:      configCollectorEndpointListSpaceSeparated,
			// TODO: move to params
			LogLevel: "SYS_DEBUG",
		})
		data["analytics-snmp-nodemanager.conf."+podIP] = nodemanagerBuffer.String()

		// TODO: commonize for all services
		var vnciniBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpVncConfig.Execute(&vnciniBuffer, struct {
			ConfigNodes   string
			ConfigApiPort string
			CAFilePath    string
		}{
			ConfigNodes:   configApiIPCommaSeparated,
			ConfigApiPort: strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:    certificates.SignerCAFilepath,
		})
		data["vnc_api_lib.ini."+podIP] = vnciniBuffer.String()
	}

	configMapInstanceDynamicConfig.Data = data

	// TODO: commonize for all services
	// update with nodemanager runner
	nmr, err := GetNodemanagerRunner()
	if err != nil {
		return err
	}
	// TODO: till not splitted to different entities
	configMapInstanceDynamicConfig.Data["analytics-snmp-nodemanager-runner.sh"] = nmr

	// update with provisioner configs
	err = UpdateProvisionerConfigMapData("analytics-snmp-provisioner", configApiIPCommaSeparated, configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateSTS creates the STS.
func (c *AnalyticsSnmp) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *AnalyticsSnmp) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "rolling")
}

//PodsCertSubjects gets list of Vrouter pods certificate subjets which can be passed to the certificate API
func (c *AnalyticsSnmp) PodsCertSubjects(podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *AnalyticsSnmp) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, &c.Spec.CommonConfiguration, request, reconcileClient)
}

// SetInstanceActive sets instance to active.
func (c *AnalyticsSnmp) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, sts, request, c)
}

// ManageNodeStatus add nodes to status
func (c *AnalyticsSnmp) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) error {

	c.Status.Nodes = podNameIPMap
	err := client.Status().Update(context.TODO(), c)
	if err != nil {
		return err
	}
	return nil
}