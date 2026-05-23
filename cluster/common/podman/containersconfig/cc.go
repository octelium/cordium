/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package containersconfig

import "github.com/pelletier/go-toml/v2"

// Config contains configuration options for container tools
type Config struct {
	// Containers specify settings that configure how containers will run ont the system
	Containers ContainersConfig `toml:"containers"`
	// Engine specifies how the container engine based on Engine will run
	Engine EngineConfig `toml:"engine"`
	// Machine specifies configurations of podman machine VMs
	Machine MachineConfig `toml:"machine"`
	// Network section defines the configuration of CNI Plugins
	Network NetworkConfig `toml:"network"`
	// Secret section defines configurations for the secret management
	Secrets SecretConfig `toml:"secrets"`
	// ConfigMap section defines configurations for the configmaps management
	ConfigMaps ConfigMapConfig `toml:"configmaps"`
}

func (c *Config) MarshalTOML() ([]byte, error) {
	return toml.Marshal(c)
}

// ContainersConfig represents the "containers" TOML config table
// containers global options for containers tools
type ContainersConfig struct {
	// Devices to add to all containers
	Devices []string `toml:"devices,omitempty"`

	// Volumes to add to all containers
	Volumes []string `toml:"volumes,omitempty"`

	// ApparmorProfile is the apparmor profile name which is used as the
	// default for the runtime.
	ApparmorProfile string `toml:"apparmor_profile,omitempty"`

	// Annotation to add to all containers
	Annotations []string `toml:"annotations,omitempty"`

	// BaseHostsFile is the path to a hosts file, the entries from this file
	// are added to the containers hosts file. As special value "image" is
	// allowed which uses the /etc/hosts file from within the image and "none"
	// which uses no base file at all. If it is empty we should default
	// to /etc/hosts.
	BaseHostsFile string `toml:"base_hosts_file,omitempty"`

	// Default way to create a cgroup namespace for the container
	CgroupNS string `toml:"cgroupns,omitempty"`

	// Default cgroup configuration
	Cgroups string `toml:"cgroups,omitempty"`

	// CgroupConf entries specifies a list of cgroup files to write to and their values. For example
	// "memory.high=1073741824" sets the memory.high limit to 1GB.
	CgroupConf []string `toml:"cgroup_conf,omitempty"`

	// Capabilities to add to all containers.
	DefaultCapabilities []string `toml:"default_capabilities,omitempty"`

	// Sysctls to add to all containers.
	DefaultSysctls []string `toml:"default_sysctls"`

	// DefaultUlimits specifies the default ulimits to apply to containers
	DefaultUlimits []string `toml:"default_ulimits,omitempty"`

	// DefaultMountsFile is the path to the default mounts file for testing
	DefaultMountsFile string `toml:"-"`

	// DNSServers set default DNS servers.
	DNSServers []string `toml:"dns_servers,omitempty"`

	// DNSOptions set default DNS options.
	DNSOptions []string `toml:"dns_options,omitempty"`

	// DNSSearches set default DNS search domains.
	DNSSearches []string `toml:"dns_searches,omitempty"`

	// EnableKeyring tells the container engines whether to create
	// a kernel keyring for use within the container
	EnableKeyring bool `toml:"keyring,omitempty"`

	// EnableLabeling tells the container engines whether to use MAC
	// Labeling to separate containers (SELinux)
	EnableLabeling bool `toml:"label,omitempty"`

	// Env is the environment variable list for container process.
	Env []string `toml:"env,omitempty"`

	// EnvHost Pass all host environment variables into the container.
	EnvHost bool `toml:"env_host,omitempty"`

	// HostContainersInternalIP is used to set a specific host.containers.internal ip.
	HostContainersInternalIP string `toml:"host_containers_internal_ip,omitempty"`

	// HTTPProxy is the proxy environment variable list to apply to container process
	HTTPProxy bool `toml:"http_proxy,omitempty"`

	// Init tells container runtimes whether to run init inside the
	// container that forwards signals and reaps processes.
	Init bool `toml:"init,omitempty"`

	// InitPath is the path for init to run if the Init bool is enabled
	InitPath string `toml:"init_path,omitempty"`

	// IPCNS way to to create a ipc namespace for the container
	IPCNS string `toml:"ipcns,omitempty"`

	// LogDriver  for the container.  For example: k8s-file and journald
	LogDriver string `toml:"log_driver,omitempty"`

	// LogSizeMax is the maximum number of bytes after which the log file
	// will be truncated. It can be expressed as a human-friendly string
	// that is parsed to bytes.
	// Negative values indicate that the log file won't be truncated.
	LogSizeMax int64 `toml:"log_size_max,omitempty,omitzero"`

	// Specifies default format tag for container log messages.
	// This is useful for creating a specific tag for container log messages.
	// Containers logs default to truncated container ID as a tag.
	LogTag string `toml:"log_tag,omitempty"`

	// NetNS indicates how to create a network namespace for the container
	NetNS string `toml:"netns,omitempty"`

	// NoHosts tells container engine whether to create its own /etc/hosts
	NoHosts bool `toml:"no_hosts,omitempty"`

	// OOMScoreAdj tunes the host's OOM preferences for containers
	// (accepts values from -1000 to 1000).
	OOMScoreAdj *int `toml:"oom_score_adj,omitempty"`

	// PidsLimit is the number of processes each container is restricted to
	// by the cgroup process number controller.
	PidsLimit int64 `toml:"pids_limit,omitempty,omitzero"`

	// PidNS indicates how to create a pid namespace for the container
	PidNS string `toml:"pidns,omitempty"`

	// Copy the content from the underlying image into the newly created
	// volume when the container is created instead of when it is started.
	// If false, the container engine will not copy the content until
	// the container is started. Setting it to true may have negative
	// performance implications.
	PrepareVolumeOnCreate bool `toml:"prepare_volume_on_create,omitempty"`

	// ReadOnly causes engine to run all containers with root file system mounted read-only
	ReadOnly bool `toml:"read_only,omitempty"`

	// SeccompProfile is the seccomp.json profile path which is used as the
	// default for the runtime.
	SeccompProfile string `toml:"seccomp_profile,omitempty"`

	// ShmSize holds the size of /dev/shm.
	ShmSize string `toml:"shm_size,omitempty"`

	// TZ sets the timezone inside the container
	TZ string `toml:"tz,omitempty"`

	// Umask is the umask inside the container.
	Umask string `toml:"umask,omitempty"`

	// UTSNS indicates how to create a UTS namespace for the container
	UTSNS string `toml:"utsns,omitempty"`

	// UserNS indicates how to create a User namespace for the container
	UserNS string `toml:"userns,omitempty"`

	// UserNSSize how many UIDs to allocate for automatically created UserNS
	// Deprecated: no user of this field is known.
	UserNSSize int `toml:"userns_size,omitempty,omitzero"`
}

// EngineConfig contains configuration options used to set up a engine runtime
type EngineConfig struct {
	// CgroupCheck indicates the configuration has been rewritten after an
	// upgrade to Fedora 31 to change the default OCI runtime for cgroupv2v2.
	CgroupCheck bool `toml:"cgroup_check,omitempty"`

	// CGroupManager is the CGroup Manager to use Valid values are "cgroupfs"
	// and "systemd".
	CgroupManager string `toml:"cgroup_manager,omitempty"`

	// NOTE: when changing this struct, make sure to update (*Config).Merge().

	// ConmonEnvVars are environment variables to pass to the Conmon binary
	// when it is launched.
	ConmonEnvVars []string `toml:"conmon_env_vars,omitempty"`

	// ConmonPath is the path to the Conmon binary used for managing containers.
	// The first path pointing to a valid file will be used.
	ConmonPath []string `toml:"conmon_path,omitempty"`

	// ConmonRsPath is the path to the Conmon-rs binary used for managing containers.
	// The first path pointing to a valid file will be used.
	ConmonRsPath []string `toml:"conmonrs_path,omitempty"`

	// CompatAPIEnforceDockerHub enforces using docker.io for completing
	// short names in Podman's compatibility REST API.  Note that this will
	// ignore unqualified-search-registries and short-name aliases defined
	// in containers-registries.conf(5).
	CompatAPIEnforceDockerHub bool `toml:"compat_api_enforce_docker_hub,omitempty"`

	// DBBackend is the database backend to be used by Podman.
	DBBackend string `toml:"database_backend,omitempty"`

	// DetachKeys is the sequence of keys used to detach a container.
	DetachKeys string `toml:"detach_keys,omitempty"`

	// EnablePortReservation determines whether engine will reserve ports on the
	// host when they are forwarded to containers. When enabled, when ports are
	// forwarded to containers, they are held open by conmon as long as the
	// container is running, ensuring that they cannot be reused by other
	// programs on the host. However, this can cause significant memory usage if
	// a container has many ports forwarded to it. Disabling this can save
	// memory.
	EnablePortReservation bool `toml:"enable_port_reservation,omitempty"`

	// Environment variables to be used when running the container engine (e.g., Podman, Buildah). For example "http_proxy=internal.proxy.company.com"
	Env []string `toml:"env,omitempty"`

	// EventsLogFilePath is where the events log is stored.
	EventsLogFilePath string `toml:"events_logfile_path,omitempty"`

	// EventsLogger determines where events should be logged.
	EventsLogger string `toml:"events_logger,omitempty"`

	// EventsContainerCreateInspectData creates a more verbose
	// container-create event which includes a JSON payload with detailed
	// information about the container.
	EventsContainerCreateInspectData bool `toml:"events_container_create_inspect_data,omitempty"`

	// graphRoot internal stores the location of the graphroot
	graphRoot string

	// configuration files. When the same filename is present in in
	// multiple directories, the file in the directory listed last in
	// this slice takes precedence.
	HooksDir []string `toml:"hooks_dir,omitempty"`

	// ImageBuildFormat (DEPRECATED) indicates the default image format to
	// building container images. Should use ImageDefaultFormat
	ImageBuildFormat string `toml:"image_build_format,omitempty"`

	// ImageDefaultTransport is the default transport method used to fetch
	// images.
	ImageDefaultTransport string `toml:"image_default_transport,omitempty"`

	// ImageParallelCopies indicates the maximum number of image layers
	// to be copied simultaneously. If this is zero, container engines
	// will fall back to containers/image defaults.
	ImageParallelCopies uint `toml:"image_parallel_copies,omitempty,omitzero"`

	// ImageDefaultFormat specified the manifest Type (oci, v2s2, or v2s1)
	// to use when pulling, pushing, building container images. By default
	// image pulled and pushed match the format of the source image.
	// Building/committing defaults to OCI.
	ImageDefaultFormat string `toml:"image_default_format,omitempty"`

	// ImageVolumeMode Tells container engines how to handle the built-in
	// image volumes.  Acceptable values are "bind", "tmpfs", and "ignore".
	ImageVolumeMode string `toml:"image_volume_mode,omitempty"`

	// InfraCommand is the command run to start up a pod infra container.
	InfraCommand string `toml:"infra_command,omitempty"`

	// InfraImage is the image a pod infra container will use to manage
	// namespaces.
	InfraImage string `toml:"infra_image,omitempty"`

	// InitPath is the path to the container-init binary.
	InitPath string `toml:"init_path,omitempty"`

	// KubeGenerateType sets the Kubernetes kind/specification to generate by default
	// with the podman kube generate command
	KubeGenerateType string `toml:"kube_generate_type,omitempty"`

	// LockType is the type of locking to use.
	LockType string `toml:"lock_type,omitempty"`

	// MachineEnabled indicates if Podman is running in a podman-machine VM
	//
	// This method is soft deprecated, use machine.IsPodmanMachine instead
	MachineEnabled bool `toml:"machine_enabled,omitempty"`

	// MultiImageArchive - if true, the container engine allows for storing
	// archives (e.g., of the docker-archive transport) with multiple
	// images.  By default, Podman creates single-image archives.
	MultiImageArchive bool `toml:"multi_image_archive,omitempty"`

	// Namespace is the engine namespace to use. Namespaces are used to create
	// scopes to separate containers and pods in the state. When namespace is
	// set, engine will only view containers and pods in the same namespace. All
	// containers and pods created will default to the namespace set here. A
	// namespace of "", the empty string, is equivalent to no namespace, and all
	// containers and pods will be visible. The default namespace is "".
	Namespace string `toml:"namespace,omitempty"`

	// NetworkCmdPath is the path to the slirp4netns binary.
	NetworkCmdPath string `toml:"network_cmd_path,omitempty"`

	// NetworkCmdOptions is the default options to pass to the slirp4netns binary.
	// For example "allow_host_loopback=true"
	NetworkCmdOptions []string `toml:"network_cmd_options,omitempty"`

	// NoPivotRoot sets whether to set no-pivot-root in the OCI runtime.
	NoPivotRoot bool `toml:"no_pivot_root,omitempty"`

	// NumLocks is the number of locks to make available for containers and
	// pods.
	NumLocks uint32 `toml:"num_locks,omitempty,omitzero"`

	// OCIRuntime is the OCI runtime to use.
	OCIRuntime string `toml:"runtime,omitempty"`

	// OCIRuntimes are the set of configured OCI runtimes (default is runc).
	OCIRuntimes map[string][]string `toml:"runtimes,omitempty"`

	// PlatformToOCIRuntime requests specific OCI runtime for a specified platform of image.
	PlatformToOCIRuntime map[string]string `toml:"platform_to_oci_runtime,omitempty"`

	// PullPolicy determines whether to pull image before creating or running a container
	// default is "missing"
	PullPolicy string `toml:"pull_policy,omitempty"`

	// Indicates whether the application should be running in Remote mode
	Remote bool `toml:"remote,omitempty"`

	// RemoteURI is deprecated, see ActiveService
	// RemoteURI containers connection information used to connect to remote system.
	RemoteURI string `toml:"remote_uri,omitempty"`

	// RemoteIdentity is deprecated, ServiceDestinations
	// RemoteIdentity key file for RemoteURI
	RemoteIdentity string `toml:"remote_identity,omitempty"`

	// ActiveService index to Destinations added v2.0.3
	ActiveService string `toml:"active_service,omitempty"`

	// ServiceDestinations mapped by service Names
	ServiceDestinations map[string]Destination `toml:"service_destinations,omitempty"`

	// SSHConfig contains the ssh config file path if not the default
	SSHConfig string `toml:"ssh_config,omitempty"`

	// RuntimePath is the path to OCI runtime binary for launching containers.
	// The first path pointing to a valid file will be used This is used only
	// when there are no OCIRuntime/OCIRuntimes defined.  It is used only to be
	// backward compatible with older versions of Podman.
	RuntimePath []string `toml:"runtime_path,omitempty"`

	// RuntimeSupportsJSON is the list of the OCI runtimes that support
	// --format=json.
	RuntimeSupportsJSON []string `toml:"runtime_supports_json,omitempty"`

	// RuntimeSupportsNoCgroups is a list of OCI runtimes that support
	// running containers without CGroups.
	RuntimeSupportsNoCgroups []string `toml:"runtime_supports_nocgroup,omitempty"`

	// RuntimeSupportsKVM is a list of OCI runtimes that support
	// KVM separation for containers.
	RuntimeSupportsKVM []string `toml:"runtime_supports_kvm,omitempty"`

	// SetOptions contains a subset of config options. It's used to indicate if
	// a given option has either been set by the user or by the parsed
	// configuration file. If not, the corresponding option might be
	// overwritten by values from the database. This behavior guarantees
	// backwards compat with older version of libpod and Podman.
	SetOptions

	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images. If left empty, the containers/image default signature
	// policy will be used.
	SignaturePolicyPath string `toml:"-"`

	// SDNotify tells container engine to allow containers to notify the host systemd of
	// readiness using the SD_NOTIFY mechanism.
	SDNotify bool `toml:"-"`

	// ServiceTimeout is the number of seconds to wait without a connection
	// before the `podman system service` times out and exits
	ServiceTimeout uint `toml:"service_timeout,omitempty,omitzero"`

	// StaticDir is the path to a persistent directory to store container
	// files.
	StaticDir string `toml:"static_dir,omitempty"`

	// StopTimeout is the number of seconds to wait for container to exit
	// before sending kill signal.
	StopTimeout uint `toml:"stop_timeout,omitempty,omitzero"`

	// ExitCommandDelay is the number of seconds to wait for the exit
	// command to be send to the API process on the server.
	ExitCommandDelay uint `toml:"exit_command_delay,omitempty,omitzero"`

	// ImageCopyTmpDir is the default location for storing temporary
	// container image content,  Can be overridden with the TMPDIR
	// environment variable.  If you specify "storage", then the
	// location of the container/storage tmp directory will be used.
	ImageCopyTmpDir string `toml:"image_copy_tmp_dir,omitempty"`

	// TmpDir is the path to a temporary directory to store per-boot container
	// files. Must be stored in a tmpfs.
	TmpDir string `toml:"tmp_dir,omitempty"`

	// VolumePath is the default location that named volumes will be created
	// under. This convention is followed by the default volume driver, but
	// may not be by other drivers.
	VolumePath string `toml:"volume_path,omitempty"`

	// VolumePluginTimeout sets the default timeout, in seconds, for
	// operations that must contact a volume plugin. Plugins are external
	// programs accessed via REST API; this sets a timeout for requests to
	// that API.
	// A value of 0 is treated as no timeout.
	VolumePluginTimeout uint `toml:"volume_plugin_timeout,omitempty,omitzero"`

	// VolumePlugins is a set of plugins that can be used as the backend for
	// Podman named volumes. Each volume is specified as a name (what Podman
	// will refer to the plugin as) mapped to a path, which must point to a
	// Unix socket that conforms to the Volume Plugin specification.
	VolumePlugins map[string]string `toml:"volume_plugins,omitempty"`

	// ChownCopiedFiles tells the container engine whether to chown files copied
	// into a container to the container's primary uid/gid.
	ChownCopiedFiles bool `toml:"chown_copied_files,omitempty"`

	// CompressionFormat is the compression format used to compress image layers.
	CompressionFormat string `toml:"compression_format,omitempty"`
}

// SetOptions contains a subset of options in a Config. It's used to indicate if
// a given option has either been set by the user or by a parsed engine
// configuration file. If not, the corresponding option might be overwritten by
// values from the database. This behavior guarantees backwards compat with
// older version of libpod and Podman.
type SetOptions struct {
	// StorageConfigRunRootSet indicates if the RunRoot has been explicitly set
	// by the config or by the user. It's required to guarantee backwards
	// compatibility with older versions of libpod for which we must query the
	// database configuration. Not included in the on-disk config.
	StorageConfigRunRootSet bool `toml:"-"`

	// StorageConfigGraphRootSet indicates if the RunRoot has been explicitly
	// set by the config or by the user. It's required to guarantee backwards
	// compatibility with older versions of libpod for which we must query the
	// database configuration. Not included in the on-disk config.
	StorageConfigGraphRootSet bool `toml:"-"`

	// StorageConfigGraphDriverNameSet indicates if the GraphDriverName has been
	// explicitly set by the config or by the user. It's required to guarantee
	// backwards compatibility with older versions of libpod for which we must
	// query the database configuration. Not included in the on-disk config.
	StorageConfigGraphDriverNameSet bool `toml:"-"`

	// StaticDirSet indicates if the StaticDir has been explicitly set by the
	// config or by the user. It's required to guarantee backwards compatibility
	// with older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	StaticDirSet bool `toml:"-"`

	// VolumePathSet indicates if the VolumePath has been explicitly set by the
	// config or by the user. It's required to guarantee backwards compatibility
	// with older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	VolumePathSet bool `toml:"-"`

	// TmpDirSet indicates if the TmpDir has been explicitly set by the config
	// or by the user. It's required to guarantee backwards compatibility with
	// older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	TmpDirSet bool `toml:"-"`
}

// NetworkConfig represents the "network" TOML config table
type NetworkConfig struct {
	// NetworkBackend determines what backend should be used for Podman's
	// networking.
	NetworkBackend string `toml:"network_backend,omitempty"`

	// CNIPluginDirs is where CNI plugin binaries are stored.
	CNIPluginDirs []string `toml:"cni_plugin_dirs,omitempty"`

	// NetavarkPluginDirs is a list of directories which contain netavark plugins.
	NetavarkPluginDirs []string `toml:"netavark_plugin_dirs,omitempty"`

	// DefaultNetwork is the network name of the default network
	// to attach pods to.
	DefaultNetwork string `toml:"default_network,omitempty"`

	// DefaultSubnet is the subnet to be used for the default network.
	// If a network with the name given in DefaultNetwork is not present
	// then a new network using this subnet will be created.
	// Must be a valid IPv4 CIDR block.
	DefaultSubnet string `toml:"default_subnet,omitempty"`

	// NetworkConfigDir is where network configuration files are stored.
	NetworkConfigDir string `toml:"network_config_dir,omitempty"`

	// DNSBindPort is the port that should be used by dns forwarding daemon
	// for netavark rootful bridges with dns enabled. This can be necessary
	// when other dns forwarders run on the machine. 53 is used if unset.
	DNSBindPort uint16 `toml:"dns_bind_port,omitempty,omitzero"`
}

// SecretConfig represents the "secret" TOML config table
type SecretConfig struct {
	// Driver specifies the secret driver to use.
	// Current valid value:
	//  * file
	//  * pass
	Driver string `toml:"driver,omitempty"`
	// Opts contains driver specific options
	Opts map[string]string `toml:"opts,omitempty"`
}

// ConfigMapConfig represents the "configmap" TOML config table
//
// revive does not like the name because the package is already called config
//
//nolint:revive
type ConfigMapConfig struct {
	// Driver specifies the configmap driver to use.
	// Current valid value:
	//  * file
	//  * pass
	Driver string `toml:"driver,omitempty"`
	// Opts contains driver specific options
	Opts map[string]string `toml:"opts,omitempty"`
}

// MachineConfig represents the "machine" TOML config table
type MachineConfig struct {
	// Number of CPU's a machine is created with.
	CPUs uint64 `toml:"cpus,omitempty,omitzero"`
	// DiskSize is the size of the disk in GB created when init-ing a podman-machine VM
	DiskSize uint64 `toml:"disk_size,omitempty,omitzero"`
	// Image is the image used when init-ing a podman-machine VM
	Image string `toml:"image,omitempty"`
	// Memory in MB a machine is created with.
	Memory uint64 `toml:"memory,omitempty,omitzero"`
	// User to use for rootless podman when init-ing a podman machine VM
	User string `toml:"user,omitempty"`
	// Volumes are host directories mounted into the VM by default.
	Volumes []string `toml:"volumes"`
	// Provider is the virtualization provider used to run podman-machine VM
	Provider string `toml:"provider,omitempty"`
}

// Destination represents destination for remote service
type Destination struct {
	// URI, required. Example: ssh://root@example.com:22/run/podman/podman.sock
	URI string `toml:"uri"`

	// Identity file with ssh key, optional
	Identity string `toml:"identity,omitempty"`

	// isMachine describes if the remote destination is a machine.
	IsMachine bool `toml:"is_machine,omitempty"`
}
