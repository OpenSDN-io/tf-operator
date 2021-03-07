package v1alpha1

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AAAMode aaa mode
// +k8s:openapi-gen=true
// +kubebuilder:validation:Enum=noauth;rbac
type AAAMode string

const (
	// AAAModeNoAuth no auth
	AAAModeNoAuth AAAMode = "no-auth"
	// AAAModeRBAC auth mode rbac
	AAAModeRBAC AAAMode = "rbac"
	// AAAModeCloudAdmin auth mode cloud-admin
	AAAModeCloudAdmin AAAMode = "cloud-admin"
)

// Config is the Schema for the configs API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=configs,scope=Namespaced
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready_Replicas",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

// ConfigSpec is the Spec for the Config API.
// +k8s:openapi-gen=true
type ConfigSpec struct {
	CommonConfiguration  PodConfiguration    `json:"commonConfiguration,omitempty"`
	ServiceConfiguration ConfigConfiguration `json:"serviceConfiguration"`
}

// ConfigConfiguration is the Spec for the Config API.
// +k8s:openapi-gen=true
type ConfigConfiguration struct {
	Containers                  []*Container `json:"containers,omitempty"`
	APIPort                     *int         `json:"apiPort,omitempty"`
	AnalyticsPort               *int         `json:"analyticsPort,omitempty"`
	CollectorPort               *int         `json:"collectorPort,omitempty"`
	ApiIntrospectPort           *int         `json:"apiIntrospectPort,omitempty"`
	SchemaIntrospectPort        *int         `json:"schemaIntrospectPort,omitempty"`
	DeviceManagerIntrospectPort *int         `json:"deviceManagerIntrospectPort,omitempty"`
	SvcMonitorIntrospectPort    *int         `json:"svcMonitorIntrospectPort,omitempty"`
	AnalyticsApiIntrospectPort  *int         `json:"analyticsIntrospectPort,omitempty"`
	CollectorIntrospectPort     *int         `json:"collectorIntrospectPort,omitempty"`
	CassandraInstance           string       `json:"cassandraInstance,omitempty"`
	ZookeeperInstance           string       `json:"zookeeperInstance,omitempty"`
	RabbitmqInstance            string       `json:"rabbitmqInstance,omitempty"`
	RabbitmqUser                string       `json:"rabbitmqUser,omitempty"`
	RabbitmqPassword            string       `json:"rabbitmqPassword,omitempty"`
	RabbitmqVhost               string       `json:"rabbitmqVhost,omitempty"`
	LogLevel                    string       `json:"logLevel,omitempty"`
	AAAMode                     AAAMode      `json:"aaaMode,omitempty"`
	Storage                     Storage      `json:"storage,omitempty"`
	FabricMgmtIP                string       `json:"fabricMgmtIP,omitempty"`
	// Time (in hours) that the analytics object and log data stays in the Cassandra database. Defaults to 48 hours.
	AnalyticsDataTTL *int `json:"analyticsDataTTL,omitempty"`
	// Time (in hours) the analytics config data entering the collector stays in the Cassandra database. Defaults to 2160 hours.
	AnalyticsConfigAuditTTL *int `json:"analyticsConfigAuditTTL,omitempty"`
	// Time to live (TTL) for statistics data in hours. Defaults to 4 hours.
	AnalyticsStatisticsTTL *int `json:"analyticsStatisticsTTL,omitempty"`
	// Time to live (TTL) for flow data in hours. Defaults to 2 hours.
	AnalyticsFlowTTL       *int                    `json:"analyticsFlowTTL,omitempty"`
	LinklocalServiceConfig *LinklocalServiceConfig `json:"linklocalServiceConfig,omitempty"`
}

// LinklocalServiceConfig is the Spec for link local coniguration
// +k8s:openapi-gen=true
type LinklocalServiceConfig struct {
	IPFabricServiceHost string  `json:"ipFabricServiceHost,omitempty"`
	IPFabricServicePort *int    `json:"ipFabricServicePort,omitempty"`
	Name                *string `json:"name,omitempty"`
	Port                *int    `json:"port,omitempty"`
	IP                  *string `json:"ip,omitempty"`
}

// ConfigStatus status of Config
// +k8s:openapi-gen=true
type ConfigStatus struct {
	Active        *bool             `json:"active,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
	Endpoint      string            `json:"endpoint,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
}

// ConfigList contains a list of Config.
// +k8s:openapi-gen=true
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

var log = logf.Log.WithName("controller_config")

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}

// InstanceConfiguration configures and updates configmaps
func (c *Config) InstanceConfiguration(configMapName string,
	request reconcile.Request,
	podList []corev1.Pod,
	client client.Client) error {

	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	configAuth := c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(
		c.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(
		c.Spec.ServiceConfiguration.ZookeeperInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(
		c.Spec.ServiceConfiguration.RabbitmqInstance, request.Namespace, client)
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

	configConfig := c.ConfigurationParameters()
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = configConfig.RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = configConfig.RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = configConfig.RabbitmqVhost
	}
	var collectorServerList, analyticsServerList, apiServerList, analyticsServerSpaceSeparatedList,
		apiServerSpaceSeparatedList, redisServerSpaceSeparatedList string
	var podIPList []string
	for _, pod := range podList {
		podIPList = append(podIPList, pod.Status.PodIP)
	}
	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	sort.SliceStable(podIPList, func(i, j int) bool { return podIPList[i] < podIPList[j] })

	collectorServerList = strings.Join(podIPList, ":"+strconv.Itoa(*configConfig.CollectorPort)+" ")
	collectorServerList = collectorServerList + ":" + strconv.Itoa(*configConfig.CollectorPort)
	analyticsServerList = strings.Join(podIPList, ",")
	apiServerList = strings.Join(podIPList, ",")
	analyticsServerSpaceSeparatedList = strings.Join(podIPList, ":"+strconv.Itoa(*configConfig.AnalyticsPort)+" ")
	analyticsServerSpaceSeparatedList = analyticsServerSpaceSeparatedList + ":" + strconv.Itoa(*configConfig.AnalyticsPort)
	apiServerSpaceSeparatedList = strings.Join(podIPList, ":"+strconv.Itoa(*configConfig.APIPort)+" ")
	apiServerSpaceSeparatedList = apiServerSpaceSeparatedList + ":" + strconv.Itoa(*configConfig.APIPort)
	redisServerSpaceSeparatedList = strings.Join(podIPList, ":6379 ") + ":6379"
	cassandraEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
	cassandraEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraEndpointList, " ")
	cassandraCQLEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.CQLPort)
	cassandraCQLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraCQLEndpointList, " ")
	rabbitMqSSLEndpointList := configtemplates.EndpointList(rabbitmqNodesInformation.ServerIPList, rabbitmqNodesInformation.Port)
	rabbitmqSSLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, " ")
	rabbitmqSSLEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, ",")
	zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
	zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")
	zookeeperEndpointListSpaceSpearated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, " ")

	var data = make(map[string]string)
	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)
		var configApiConfigBuffer bytes.Buffer
		err = configtemplates.ConfigAPIConfig.Execute(&configApiConfigBuffer, struct {
			PodIP                    string
			ListenAddress            string
			ListenPort               string
			InstrospectListenAddress string
			ApiIntrospectPort        string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			CollectorServerList      string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			AuthMode                 AuthenticationMode
			AAAMode                  AAAMode
			LogLevel                 string
			CAFilePath               string
		}{
			PodIP:                    podIP,
			ListenAddress:            podIP,
			ListenPort:               strconv.Itoa(*configConfig.APIPort),
			InstrospectListenAddress: instrospectListenAddress,
			ApiIntrospectPort:        strconv.Itoa(*configConfig.ApiIntrospectPort),
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:      collectorServerList,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			AuthMode:                 c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			AAAMode:                  configConfig.AAAMode,
			LogLevel:                 configConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["api."+podIP] = configApiConfigBuffer.String()

		var vncApiConfigBuffer bytes.Buffer
		err = configtemplates.ConfigAPIVNC.Execute(&vncApiConfigBuffer, struct {
			PodIP                  string
			APIServerList          string
			APIServerPort          string
			AuthMode               AuthenticationMode
			CAFilePath             string
			KeystoneAddress        string
			KeystonePort           *int
			KeystoneUserDomainName string
			KeystoneAuthProtocol   string
		}{
			PodIP:                  podIP,
			APIServerList:          apiServerList,
			APIServerPort:          strconv.Itoa(*configConfig.APIPort),
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			CAFilePath:             certificates.SignerCAFilepath,
			KeystoneAddress:        configAuth.Address,
			KeystonePort:           configAuth.Port,
			KeystoneUserDomainName: configAuth.UserDomainName,
			KeystoneAuthProtocol:   configAuth.AuthProtocol,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+podIP] = vncApiConfigBuffer.String()

		fabricMgmtIP := podIP
		if c.Spec.ServiceConfiguration.FabricMgmtIP != "" {
			fabricMgmtIP = c.Spec.ServiceConfiguration.FabricMgmtIP
		}

		var configDevicemanagerConfigBuffer bytes.Buffer
		err = configtemplates.ConfigDeviceManagerConfig.Execute(&configDevicemanagerConfigBuffer, struct {
			PodIP                       string
			ListenAddress               string
			InstrospectListenAddress    string
			DeviceManagerIntrospectPort string
			ApiServerList               string
			AnalyticsServerList         string
			CassandraServerList         string
			ZookeeperServerList         string
			RabbitmqServerList          string
			CollectorServerList         string
			RabbitmqUser                string
			RabbitmqPassword            string
			RabbitmqVhost               string
			LogLevel                    string
			FabricMgmtIP                string
			CAFilePath                  string
		}{
			PodIP:                       podIP,
			ListenAddress:               podIP,
			InstrospectListenAddress:    instrospectListenAddress,
			DeviceManagerIntrospectPort: strconv.Itoa(*configConfig.DeviceManagerIntrospectPort),
			ApiServerList:               apiServerList,
			AnalyticsServerList:         analyticsServerList,
			CassandraServerList:         cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:         zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:          rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:         collectorServerList,
			RabbitmqUser:                rabbitmqSecretUser,
			RabbitmqPassword:            rabbitmqSecretPassword,
			RabbitmqVhost:               rabbitmqSecretVhost,
			LogLevel:                    configConfig.LogLevel,
			FabricMgmtIP:                fabricMgmtIP,
			CAFilePath:                  certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["devicemanager."+podIP] = configDevicemanagerConfigBuffer.String()

		var fabricAnsibleConfigBuffer bytes.Buffer
		err = configtemplates.FabricAnsibleConf.Execute(&fabricAnsibleConfigBuffer, struct {
			PodIP               string
			CollectorServerList string
			LogLevel            string
			CAFilePath          string
		}{
			PodIP:               podIP,
			CollectorServerList: collectorServerList,
			LogLevel:            configConfig.LogLevel,
			CAFilePath:          certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["contrail-fabric-ansible.conf."+podIP] = fabricAnsibleConfigBuffer.String()

		var configKeystoneAuthConfBuffer bytes.Buffer
		err = configtemplates.ConfigKeystoneAuthConf.Execute(&configKeystoneAuthConfBuffer, struct {
			AdminUsername             string
			AdminPassword             *string
			KeystoneAddress           string
			KeystonePort              *int
			KeystoneAuthProtocol      string
			KeystoneUserDomainName    string
			KeystoneProjectDomainName string
			KeystoneRegion            string
			CAFilePath                string
		}{
			AdminUsername:             configAuth.AdminUsername,
			AdminPassword:             configAuth.AdminPassword,
			KeystoneAddress:           configAuth.Address,
			KeystonePort:              configAuth.Port,
			KeystoneAuthProtocol:      configAuth.AuthProtocol,
			KeystoneUserDomainName:    configAuth.UserDomainName,
			KeystoneProjectDomainName: configAuth.ProjectDomainName,
			KeystoneRegion:            configAuth.Region,
			CAFilePath:                certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["contrail-keystone-auth.conf."+podIP] = configKeystoneAuthConfBuffer.String()

		data["dnsmasq."+podIP] = configtemplates.ConfigDNSMasqConfig
		data["dnsmasq_base."+podIP] = configtemplates.ConfigDNSMasqBaseConfig

		var configSchematransformerConfigBuffer bytes.Buffer
		err = configtemplates.ConfigSchematransformerConfig.Execute(&configSchematransformerConfigBuffer, struct {
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			SchemaIntrospectPort     string
			ApiServerList            string
			AnalyticsServerList      string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			CollectorServerList      string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			LogLevel                 string
			CAFilePath               string
		}{
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			SchemaIntrospectPort:     strconv.Itoa(*configConfig.SchemaIntrospectPort),
			ApiServerList:            apiServerList,
			AnalyticsServerList:      analyticsServerList,
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:      collectorServerList,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			LogLevel:                 configConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["schematransformer."+podIP] = configSchematransformerConfigBuffer.String()

		var configServicemonitorConfigBuffer bytes.Buffer
		err = configtemplates.ConfigServicemonitorConfig.Execute(&configServicemonitorConfigBuffer, struct {
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			SvcMonitorIntrospectPort string
			ApiServerList            string
			AnalyticsServerList      string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			CollectorServerList      string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			AAAMode                  AAAMode
			LogLevel                 string
			CAFilePath               string
		}{
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			SvcMonitorIntrospectPort: strconv.Itoa(*configConfig.SvcMonitorIntrospectPort),
			ApiServerList:            apiServerList,
			AnalyticsServerList:      analyticsServerSpaceSeparatedList,
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:      collectorServerList,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			AAAMode:                  configConfig.AAAMode,
			LogLevel:                 configConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["servicemonitor."+podIP] = configServicemonitorConfigBuffer.String()

		var configAnalyticsapiConfigBuffer bytes.Buffer
		err = configtemplates.ConfigAnalyticsapiConfig.Execute(&configAnalyticsapiConfigBuffer, struct {
			PodIP                      string
			ListenAddress              string
			InstrospectListenAddress   string
			AnalyticsApiIntrospectPort string
			ApiServerList              string
			AnalyticsServerList        string
			CassandraServerList        string
			ZookeeperServerList        string
			RabbitmqServerList         string
			CollectorServerList        string
			RedisServerList            string
			RabbitmqUser               string
			RabbitmqPassword           string
			RabbitmqVhost              string
			AuthMode                   string
			AAAMode                    AAAMode
			CAFilePath                 string
			LogLevel                   string
		}{
			PodIP:                      podIP,
			ListenAddress:              podIP,
			InstrospectListenAddress:   instrospectListenAddress,
			AnalyticsApiIntrospectPort: strconv.Itoa(*configConfig.AnalyticsApiIntrospectPort),
			ApiServerList:              apiServerSpaceSeparatedList,
			AnalyticsServerList:        analyticsServerSpaceSeparatedList,
			CassandraServerList:        cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:        zookeeperEndpointListSpaceSpearated,
			RabbitmqServerList:         rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:        collectorServerList,
			RedisServerList:            redisServerSpaceSeparatedList,
			RabbitmqUser:               rabbitmqSecretUser,
			RabbitmqPassword:           rabbitmqSecretPassword,
			RabbitmqVhost:              rabbitmqSecretVhost,
			AAAMode:                    configConfig.AAAMode,
			CAFilePath:                 certificates.SignerCAFilepath,
			LogLevel:                   configConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["analyticsapi."+podIP] = configAnalyticsapiConfigBuffer.String()

		var configCollectorConfigBuffer bytes.Buffer
		err = configtemplates.ConfigCollectorConfig.Execute(&configCollectorConfigBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CollectorIntrospectPort  string
			ApiServerList            string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			LogLevel                 string
			CAFilePath               string
			AnalyticsDataTTL         string
			AnalyticsConfigAuditTTL  string
			AnalyticsStatisticsTTL   string
			AnalyticsFlowTTL         string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorIntrospectPort:  strconv.Itoa(*configConfig.CollectorIntrospectPort),
			ApiServerList:            apiServerSpaceSeparatedList,
			CassandraServerList:      cassandraCQLEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			LogLevel:                 configConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
			AnalyticsDataTTL:         strconv.Itoa(*configConfig.AnalyticsDataTTL),
			AnalyticsConfigAuditTTL:  strconv.Itoa(*configConfig.AnalyticsConfigAuditTTL),
			AnalyticsStatisticsTTL:   strconv.Itoa(*configConfig.AnalyticsStatisticsTTL),
			AnalyticsFlowTTL:         strconv.Itoa(*configConfig.AnalyticsFlowTTL),
		})
		if err != nil {
			panic(err)
		}
		data["collector."+podIP] = configCollectorConfigBuffer.String()

		var configQueryEngineConfigBuffer bytes.Buffer
		err = configtemplates.ConfigQueryEngineConfig.Execute(&configQueryEngineConfigBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CassandraServerList      string
			CollectorServerList      string
			RedisServerList          string
			CAFilePath               string
			AnalyticsDataTTL         string
			LogLevel                 string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CassandraServerList:      cassandraCQLEndpointListSpaceSeparated,
			CollectorServerList:      collectorServerList,
			RedisServerList:          redisServerSpaceSeparatedList,
			CAFilePath:               certificates.SignerCAFilepath,
			AnalyticsDataTTL:         strconv.Itoa(*configConfig.AnalyticsDataTTL),
			LogLevel:                 configConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["queryengine."+podIP] = configQueryEngineConfigBuffer.String()

		var configNodemanagerconfigConfigBuffer bytes.Buffer
		err = configtemplates.ConfigNodemanagerConfigConfig.Execute(&configNodemanagerconfigConfigBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CollectorServerList      string
			CassandraPort            string
			CassandraJmxPort         string
			CAFilePath               string
			LogLevel                 string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServerList:      collectorServerList,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 configConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["config-nodemgr.conf."+podIP] = configNodemanagerconfigConfigBuffer.String()
		// empty env as no db tracking
		data["config-nodemgr.env."+podIP] = ""

		var configNodemanageranalyticsConfigBuffer bytes.Buffer
		err = configtemplates.ConfigNodemanagerAnalyticsConfig.Execute(&configNodemanageranalyticsConfigBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CollectorServerList      string
			CassandraPort            string
			CassandraJmxPort         string
			CAFilePath               string
			LogLevel                 string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServerList:      collectorServerList,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 configConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["analytics-nodemgr.conf."+podIP] = configNodemanageranalyticsConfigBuffer.String()
		// empty env as no db tracking
		data["analytics-nodemgr.env."+podIP] = ""

		var configStunnelConfigBuffer bytes.Buffer
		err = configtemplates.ConfigStunnelConfig.Execute(&configStunnelConfigBuffer, struct {
			RedisListenAddress string
			RedisServerPort    string
		}{
			RedisListenAddress: podIP,
			RedisServerPort:    "6379",
		})
		if err != nil {
			panic(err)
		}
		data["stunnel."+podIP] = configStunnelConfigBuffer.String()
	}

	configMapInstanceDynamicConfig.Data = data

	// update with nodemanager runner
	nmr := GetNodemanagerRunner()
	configMapInstanceDynamicConfig.Data["config-nodemanager-runner.sh"] = nmr
	// TODO: till not splitted to different entities
	configMapInstanceDynamicConfig.Data["analytics-nodemanager-runner.sh"] = nmr

	// update with provisioner configs
	UpdateProvisionerConfigMapData("config-provisioner", apiServerList, configMapInstanceDynamicConfig)

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateConfigMap makes default empty ConfigMap
func (c *Config) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"config",
		c)
}

// CreateSecret creates a secret.
func (c *Config) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"config",
		c)
}

// PrepareSTS prepares the intented statefulset for the config object
func (c *Config) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "config", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the config statefulset
func (c *Config) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Config) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

//CreateSTS creates the STS
func (c *Config) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

//UpdateSTS updates the STS
func (c *Config) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "deleteFirst")
}

// SetInstanceActive sets the Config instance to active
func (c *Config) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	if err := client.Get(context.TODO(), types.NamespacedName{Name: sts.Name, Namespace: request.Namespace}, sts); err != nil {
		return err
	}
	*activeStatus = sts.Status.ReadyReplicas >= *sts.Spec.Replicas/2+1
	if err := client.Status().Update(context.TODO(), c); err != nil {
		return err
	}
	return nil
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Config) PodIPListAndIPMapFromInstance(request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance("config", &c.Spec.CommonConfiguration, request, reconcileClient)
}

//PodsCertSubjects gets list of Config pods certificate subjets which can be passed to the certificate API
func (c *Config) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetPodsToReady set pods ready
func (c *Config) SetPodsToReady(podIPList []corev1.Pod, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// ManageNodeStatus updates nodes in status
func (c *Config) ManageNodeStatus(podNameIPMap map[string]string, client client.Client) error {
	c.Status.Nodes = podNameIPMap
	err := client.Status().Update(context.TODO(), c)
	if err != nil {
		return err
	}
	return nil
}

// IsActive returns true if instance is active
func (c *Config) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// ConfigurationParameters create config struct
func (c *Config) ConfigurationParameters() ConfigConfiguration {
	configConfiguration := ConfigConfiguration{}
	var apiPort int
	var analyticsPort int
	var collectorPort int
	var rabbitmqUser string
	var rabbitmqPassword string
	var rabbitmqVhost string
	var logLevel string

	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	} else {
		logLevel = LogLevel
	}
	configConfiguration.LogLevel = logLevel
	if c.Spec.ServiceConfiguration.APIPort != nil {
		apiPort = *c.Spec.ServiceConfiguration.APIPort
	} else {
		apiPort = ConfigApiPort
	}
	configConfiguration.APIPort = &apiPort

	if c.Spec.ServiceConfiguration.AnalyticsPort != nil {
		analyticsPort = *c.Spec.ServiceConfiguration.AnalyticsPort
	} else {
		analyticsPort = AnalyticsApiPort
	}
	configConfiguration.AnalyticsPort = &analyticsPort

	if c.Spec.ServiceConfiguration.CollectorPort != nil {
		collectorPort = *c.Spec.ServiceConfiguration.CollectorPort
	} else {
		collectorPort = CollectorPort
	}
	configConfiguration.CollectorPort = &collectorPort

	var apiIntrospectPort int
	if c.Spec.ServiceConfiguration.ApiIntrospectPort != nil {
		apiIntrospectPort = *c.Spec.ServiceConfiguration.ApiIntrospectPort
	} else {
		apiIntrospectPort = ConfigApiIntrospectPort
	}
	configConfiguration.ApiIntrospectPort = &apiIntrospectPort

	var schemaIntrospectPort int
	if c.Spec.ServiceConfiguration.SchemaIntrospectPort != nil {
		schemaIntrospectPort = *c.Spec.ServiceConfiguration.SchemaIntrospectPort
	} else {
		schemaIntrospectPort = ConfigSchemaIntrospectPort
	}
	configConfiguration.SchemaIntrospectPort = &schemaIntrospectPort

	var deviceManagerIntrospectPort int
	if c.Spec.ServiceConfiguration.DeviceManagerIntrospectPort != nil {
		deviceManagerIntrospectPort = *c.Spec.ServiceConfiguration.DeviceManagerIntrospectPort
	} else {
		deviceManagerIntrospectPort = ConfigDeviceManagerIntrospectPort
	}
	configConfiguration.DeviceManagerIntrospectPort = &deviceManagerIntrospectPort

	var svcMonitorIntrospectPort int
	if c.Spec.ServiceConfiguration.SvcMonitorIntrospectPort != nil {
		svcMonitorIntrospectPort = *c.Spec.ServiceConfiguration.SvcMonitorIntrospectPort
	} else {
		svcMonitorIntrospectPort = ConfigSvcMonitorIntrospectPort
	}
	configConfiguration.SvcMonitorIntrospectPort = &svcMonitorIntrospectPort

	var analyticsApiIntrospectPort int
	if c.Spec.ServiceConfiguration.AnalyticsApiIntrospectPort != nil {
		analyticsApiIntrospectPort = *c.Spec.ServiceConfiguration.AnalyticsApiIntrospectPort
	} else {
		analyticsApiIntrospectPort = AnalyticsApiIntrospectPort
	}
	configConfiguration.AnalyticsApiIntrospectPort = &analyticsApiIntrospectPort

	var collectorIntrospectPort int
	if c.Spec.ServiceConfiguration.CollectorIntrospectPort != nil {
		collectorIntrospectPort = *c.Spec.ServiceConfiguration.CollectorIntrospectPort
	} else {
		collectorIntrospectPort = CollectorIntrospectPort
	}
	configConfiguration.CollectorIntrospectPort = &collectorIntrospectPort

	if c.Spec.ServiceConfiguration.RabbitmqUser != "" {
		rabbitmqUser = c.Spec.ServiceConfiguration.RabbitmqUser
	} else {
		rabbitmqUser = RabbitmqUser
	}
	configConfiguration.RabbitmqUser = rabbitmqUser

	if c.Spec.ServiceConfiguration.RabbitmqPassword != "" {
		rabbitmqPassword = c.Spec.ServiceConfiguration.RabbitmqPassword
	} else {
		rabbitmqPassword = RabbitmqPassword
	}
	configConfiguration.RabbitmqPassword = rabbitmqPassword

	if c.Spec.ServiceConfiguration.RabbitmqVhost != "" {
		rabbitmqVhost = c.Spec.ServiceConfiguration.RabbitmqVhost
	} else {
		rabbitmqVhost = RabbitmqVhost
	}
	configConfiguration.RabbitmqVhost = rabbitmqVhost

	configConfiguration.AAAMode = c.Spec.ServiceConfiguration.AAAMode
	if configConfiguration.AAAMode == "" {
		configConfiguration.AAAMode = AAAModeNoAuth
		ap := c.Spec.CommonConfiguration.AuthParameters
		if ap != nil && ap.AuthMode == AuthenticationModeKeystone {
			configConfiguration.AAAMode = AAAModeRBAC
		}
	}

	var analyticsDataTTL int
	if c.Spec.ServiceConfiguration.AnalyticsDataTTL != nil {
		analyticsDataTTL = *c.Spec.ServiceConfiguration.AnalyticsDataTTL
	} else {
		analyticsDataTTL = AnalyticsDataTTL
	}
	configConfiguration.AnalyticsDataTTL = &analyticsDataTTL

	var analyticsConfigAuditTTL int
	if c.Spec.ServiceConfiguration.AnalyticsConfigAuditTTL != nil {
		analyticsConfigAuditTTL = *c.Spec.ServiceConfiguration.AnalyticsConfigAuditTTL
	} else {
		analyticsConfigAuditTTL = AnalyticsConfigAuditTTL
	}
	configConfiguration.AnalyticsConfigAuditTTL = &analyticsConfigAuditTTL

	var analyticsStatisticsTTL int
	if c.Spec.ServiceConfiguration.AnalyticsStatisticsTTL != nil {
		analyticsStatisticsTTL = *c.Spec.ServiceConfiguration.AnalyticsStatisticsTTL
	} else {
		analyticsStatisticsTTL = AnalyticsStatisticsTTL
	}
	configConfiguration.AnalyticsStatisticsTTL = &analyticsStatisticsTTL

	var analyticsFlowTTL int
	if c.Spec.ServiceConfiguration.AnalyticsFlowTTL != nil {
		analyticsFlowTTL = *c.Spec.ServiceConfiguration.AnalyticsFlowTTL
	} else {
		analyticsFlowTTL = AnalyticsFlowTTL
	}
	configConfiguration.AnalyticsFlowTTL = &analyticsFlowTTL

	if c.Spec.ServiceConfiguration.LinklocalServiceConfig != nil {
		configConfiguration.LinklocalServiceConfig = c.Spec.ServiceConfiguration.LinklocalServiceConfig
		if configConfiguration.LinklocalServiceConfig.Name == nil {
			name := LinklocalServiceName
			configConfiguration.LinklocalServiceConfig.Name = &name
		}
		if configConfiguration.LinklocalServiceConfig.Port == nil {
			port := LinklocalServicePort
			configConfiguration.LinklocalServiceConfig.Port = &port
		}
		if configConfiguration.LinklocalServiceConfig.IP == nil {
			ip := LinklocalServiceIp
			configConfiguration.LinklocalServiceConfig.IP = &ip
		}
		if configConfiguration.LinklocalServiceConfig.IPFabricServicePort == nil {
			port := IpfabricServicePort
			configConfiguration.LinklocalServiceConfig.IPFabricServicePort = &port
		}
	}

	return configConfiguration

}

// SetEndpointInStatus updates Endpoint in status
func (c *Config) SetEndpointInStatus(client client.Client, clusterIP string) error {
	c.Status.Endpoint = clusterIP
	err := client.Status().Update(context.TODO(), c)
	return err
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Config) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
