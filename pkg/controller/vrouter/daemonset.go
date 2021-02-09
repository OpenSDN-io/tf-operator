package vrouter

import (
	"github.com/Juniper/contrail-operator/pkg/certificates"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//GetDaemonset returns DaemonSet object for vRouter
func GetDaemonset(cloudOrchestrator string) *apps.DaemonSet {
	var labelsMountPermission int32 = 0644
	var trueVal = true

	envList := []corev1.EnvVar{
		{
			Name:  "VENDOR_DOMAIN",
			Value: "io.tungsten",
		},
		{
			Name:  "NODE_TYPE",
			Value: "vrouter",
		},
		{
			Name: "POD_IP",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			// TODO: remove after tf-container-builder support PROVISION_HOSTNAME
			Name: "VROUTER_HOSTNAME",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.annotations['hostname']",
				},
			},
		},
		{
			Name: "PROVISION_HOSTNAME",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.annotations['hostname']",
				},
			},
		},
		{
			Name:  "CLOUD_ORCHESTRATOR",
			Value: cloudOrchestrator,
		},
		{
			Name: "PHYSICAL_INTERFACE",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.annotations['physicalInterface']",
				},
			},
		},
		{
			Name:  "INTROSPECT_SSL_ENABLE",
			Value: "True",
		},
		{
			Name:  "SSL_ENABLE",
			Value: "True",
		},
	}

	envListNodeInit := append(envList,
		corev1.EnvVar{
			Name:  "SERVER_CA_CERTFILE",
			Value: certificates.SignerCAFilepath,
		},
		corev1.EnvVar{
			Name:  "SERVER_CERTFILE",
			Value: "/etc/certificates/server-${POD_IP}.crt",
		},
		corev1.EnvVar{
			Name:  "SERVER_KEYFILE",
			Value: "/etc/certificates/server-key-${POD_IP}.pem",
		},
	)

	var podInitContainers = []core.Container{
		{
			Name:  "init",
			Image: "busybox:latest",
			Command: []string{
				"sh",
				"-c",
				"until grep ready /tmp/podinfo/pod_labels > /dev/null 2>&1; do sleep 1; done",
			},
			Env: envList,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "status",
					MountPath: "/tmp/podinfo",
				},
			},
		},
		{
			Name:  "nodeinit",
			Image: "tungstenfabric/contrail-node-init:latest",
			Env:   envListNodeInit,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "host-usr-bin",
					MountPath: "/host/usr/bin",
				},
			},
			SecurityContext: &core.SecurityContext{
				Privileged: &trueVal,
			},
		},
		{
			Name:  "vrouterkernelinit",
			Image: "tungstenfabric/contrail-vrouter-kernel-init:latest",
			Env:   envList,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "network-scripts",
					MountPath: "/etc/sysconfig/network-scripts",
				},
				{
					Name:      "host-usr-bin",
					MountPath: "/host/bin",
				},
				{
					Name:      "usr-src",
					MountPath: "/usr/src",
				},
				{
					Name:      "lib-modules",
					MountPath: "/lib/modules",
				},
			},
			SecurityContext: &core.SecurityContext{
				Privileged: &trueVal,
			},
		},
		{
			Name:  "vroutercni",
			Image: "tungstenfabric/contrail-kubernetes-cni-init:latest",
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "var-lib-contrail",
					MountPath: "/var/lib/contrail",
				},
				{
					Name:      "vrouter-logs",
					MountPath: "/var/log/contrail",
				},
				{
					Name:      "cni-config-files",
					MountPath: "/host/etc_cni",
				},
				{
					Name:      "etc-kubernetes-cni",
					MountPath: "/etc/kubernetes/cni",
				},
				{
					Name:      "cni-bin",
					MountPath: "/host/opt_cni_bin",
				},
				{
					Name:      "multus-cni",
					MountPath: "/var/run/multus",
				},
			},
		},
	}

	var podContainers = []core.Container{
		{
			Name:  "provisioner",
			Image: "tungstenfabric/contrail-provisioner:latest",
			Env:   envList,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "vrouter-logs",
					MountPath: "/var/log/contrail",
				},
			},
		},
		{
			Name:  "nodemanager",
			Image: "tungstenfabric/contrail-nodemgr:latest",
			Env:   envList,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "vrouter-logs",
					MountPath: "/var/log/contrail",
				},
				{
					Name:      "var-run",
					MountPath: "/var/run",
				},
				{
					Name:      "var-crashes",
					MountPath: "/var/crashes",
				},
			},
		},
		{
			Name:  "vrouteragent",
			Image: "tungstenfabric/contrail-vrouter-agent:latest",
			Env:   envList,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "vrouter-agent-logs",
					MountPath: "/var/log/contrail",
				},
				{
					Name:      "dev",
					MountPath: "/dev",
				},
				{
					Name:      "network-scripts",
					MountPath: "/etc/sysconfig/network-scripts",
				},
				{
					Name:      "host-usr-bin",
					MountPath: "/host/bin",
				},
				{
					Name:      "usr-src",
					MountPath: "/usr/src",
				},
				{
					Name:      "lib-modules",
					MountPath: "/lib/modules",
				},
				{
					Name:      "var-run",
					MountPath: "/var/run",
				},
				{
					Name:      "var-lib-contrail",
					MountPath: "/var/lib/contrail",
				},
				{
					Name:      "var-crashes",
					MountPath: "/var/crashes",
				},
			},
			SecurityContext: &core.SecurityContext{
				Privileged: &trueVal,
			},
			Lifecycle: &core.Lifecycle{
				PreStop: &core.Handler{
					Exec: &core.ExecAction{
						Command: []string{"/clean-up.sh"},
					},
				},
			},
		},
	}

	var podVolumes = []core.Volume{
		{
			// for agent as it use tf-container-builder logic
			Name: "vrouter-agent-logs",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/log/contrail",
				},
			},
		},
		{
			// for nodemgr and provisioner
			Name: "vrouter-logs",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/log/contrail/vrouter-agent",
				},
			},
		},
		{
			Name: "var-run",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/run",
				},
			},
		},
		{
			Name: "host-usr-bin",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/usr/bin",
				},
			},
		},
		{
			Name: "var-crashes",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/crashes/contrail",
				},
			},
		},
		{
			Name: "var-lib-contrail",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/lib/contrail",
				},
			},
		},
		{
			Name: "lib-modules",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/lib/modules",
				},
			},
		},
		{
			Name: "usr-src",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/usr/src",
				},
			},
		},
		{
			Name: "network-scripts",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/etc/sysconfig/network-scripts",
				},
			},
		},
		{
			Name: "dev",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/dev",
				},
			},
		},
		{
			Name: "cni-config-files",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/etc/cni",
				},
			},
		},
		{
			Name: "cni-bin",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					// TODO: allow to overwrite via params
					Path: "/opt/cni/bin",
				},
			},
		},
		{
			Name: "etc-kubernetes-cni",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/etc/kubernetes/cni",
				},
			},
		},
		{
			Name: "multus-cni",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/run/multus",
				},
			},
		},
		{
			Name: "status",
			VolumeSource: core.VolumeSource{
				DownwardAPI: &core.DownwardAPIVolumeSource{
					Items: []core.DownwardAPIVolumeFile{
						{
							Path: "pod_labels",
							FieldRef: &core.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.labels",
							},
						},
						{
							Path: "pod_labelsx",
							FieldRef: &core.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.labels",
							},
						},
					},
					DefaultMode: &labelsMountPermission,
				},
			},
		},
	}

	var podTolerations = []core.Toleration{
		{
			Operator: "Exists",
			Effect:   "NoSchedule",
		},
		{
			Operator: "Exists",
			Effect:   "NoExecute",
		},
	}

	var podSpec = core.PodSpec{
		Volumes:        podVolumes,
		InitContainers: podInitContainers,
		Containers:     podContainers,
		RestartPolicy:  "Always",
		DNSPolicy:      "ClusterFirstWithHostNet",
		HostNetwork:    true,
		Tolerations:    podTolerations,
	}

	var daemonSetSelector = meta.LabelSelector{
		MatchLabels: map[string]string{"app": "vrouter"},
	}

	var daemonsetTemplate = core.PodTemplateSpec{
		ObjectMeta: meta.ObjectMeta{},
		Spec:       podSpec,
	}

	var daemonSet = apps.DaemonSet{
		TypeMeta: meta.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      "vrouter",
			Namespace: "default",
		},
		Spec: apps.DaemonSetSpec{
			Selector: &daemonSetSelector,
			Template: daemonsetTemplate,
		},
	}

	return &daemonSet
}
