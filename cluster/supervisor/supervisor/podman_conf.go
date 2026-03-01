/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package supervisor

const podmanConfContainers = `
# The containers configuration file specifies all of the available configuration
# command-line options/flags for container engine tools like Podman & Buildah,
# but in a TOML format that can be easily modified and versioned.

# Please refer to containers.conf(5) for details of all configuration options.
# Not all container engines implement all of the options.
# All of the options have hard coded defaults and these options will override
# the built in defaults. Users can then override these options via the command
# line. Container engines will read containers.conf files in up to three
# locations in the following order:
#  1. /usr/share/containers/containers.conf
#  2. /etc/containers/containers.conf
#  3. $HOME/.config/containers/containers.conf (Rootless containers ONLY)
#  Items specified in the latter containers.conf, if they exist, override the
# previous containers.conf settings, or the default settings.

[containers]

# List of annotation. Specified as
# "key = value"
# If it is empty or commented out, no annotations will be added
#
#annotations = []

# Used to change the name of the default AppArmor profile of container engine.
#
#apparmor_profile = "container-default"

# The hosts entries from the base hosts file are added to the containers hosts
# file. This must be either an absolute path or as special values "image" which
# uses the hosts file from the container image or "none" which means
# no base hosts file is used. The default is "" which will use /etc/hosts.
#
#base_hosts_file = ""


#cgroupns = "private"

# Control container cgroup configuration
# Determines  whether  the  container will create CGroups.
# Options are:

#cgroups = "enabled"

# List of default capabilities for containers. If it is empty or commented out,
# the default capabilities defined in the container engine will be added.
#
default_capabilities = [
  "CHOWN",
  "DAC_OVERRIDE",
  "FOWNER",
  "FSETID",
  "KILL",
  "NET_BIND_SERVICE",
  "SETFCAP",
  "SETGID",
  "SETPCAP",
  "SETUID",
  "SYS_CHROOT"
]

# A list of sysctls to be set in containers by default,
# specified as "name=value",
# for example:"net.ipv4.ping_group_range=0 0".
#
# default_sysctls = [
#  "net.ipv4.ping_group_range=0 0",
# ]
default_sysctls = []

# A list of ulimits to be set in containers by default, specified as
# "<ulimit name>=<soft limit>:<hard limit>", for example:
# "nofile=1024:2048"
# See setrlimit(2) for a list of resource names.
# Any limit not specified here will be inherited from the process launching the
# container engine.
# Ulimits has limits for non privileged container engines.
#
#default_ulimits = [
#  "nofile=1280:2560",
#]

# List of devices. Specified as
# "<device-on-host>:<device-on-container>:<permissions>", for example:
# "/dev/sdc:/dev/xvdc:rwm".
# If it is empty or commented out, only the default devices will be used
#
#devices = []

# List of default DNS options to be added to /etc/resolv.conf inside of the container.
#
#dns_options = []

# List of default DNS search domains to be added to /etc/resolv.conf inside of the container.
#
#dns_searches = []

# Set default DNS servers.
# This option can be used to override the DNS configuration passed to the
# container. The special value "none" can be specified to disable creation of
# /etc/resolv.conf in the container.
# The /etc/resolv.conf file in the image will be used without changes.
#
#dns_servers = []

# Environment variable list for the conmon process; used for passing necessary
# environment variables to conmon or the runtime.
#
#env = [
#  "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
#  "TERM=xterm",
#]

# Pass all host environment variables into the container.
#
#env_host = false

# Set the ip for the host.containers.internal entry in the containers /etc/hosts
# file. This can be set to "none" to disable adding this entry. By default it
# will automatically choose the host ip.
#
# NOTE: When using podman machine this entry will never be added to the containers
# hosts file instead the gvproxy dns resolver will resolve this hostname. Therefore
# it is not possible to disable the entry in this case.
#
#host_containers_internal_ip = ""

# Default proxy environment variables passed into the container.
# The environment variables passed in include:
# http_proxy, https_proxy, ftp_proxy, no_proxy, and the upper case versions of
# these. This option is needed when host system uses a proxy but container
# should not use proxy. Proxy environment variables specified for the container
# in any other way will override the values passed from the host.
#
#http_proxy = true

# Run an init inside the container that forwards signals and reaps processes.
#
#init = false

# Container init binary, if init=true, this is the init binary to be used for containers.
#
#init_path = "/usr/libexec/podman/catatonit"

# Default way to to create an IPC namespace (POSIX SysV IPC) for the container
# Options are:
#  "host"     Share host IPC Namespace with the container.
#  "none"     Create shareable IPC Namespace for the container without a private /dev/shm.
#  "private"  Create private IPC Namespace for the container, other containers are not allowed to share it.
#  "shareable" Create shareable IPC Namespace for the container.
#
#ipcns = "shareable"

# keyring tells the container engine whether to create
# a kernel keyring for use within the container.
#
#keyring = true

# label tells the container engine whether to use container separation using
# MAC(SELinux) labeling or not.
# The label flag is ignored on label disabled systems.
#
#label = true

# Logging driver for the container. Available options: k8s-file and journald.
#
#log_driver = "k8s-file"

# Maximum size allowed for the container log file. Negative numbers indicate
# that no size limit is imposed. If positive, it must be >= 8192 to match or
# exceed conmon's read buffer. The file is truncated and re-opened so the
# limit is never exceeded.
#
#log_size_max = -1

# Specifies default format tag for container log messages.
# This is useful for creating a specific tag for container log messages.
# Containers logs default to truncated container ID as a tag.
#
#log_tag = ""

# Default way to to create a Network namespace for the container
# Options are:

#netns = "private"

# Create /etc/hosts for the container.  By default, container engine manage
# /etc/hosts, automatically adding  the container's  own  IP  address.
#
#no_hosts = false

# Default way to to create a PID namespace for the container
# Options are:

#pidns = "private"

# Maximum number of processes allowed in a container.
#
#pids_limit = 2048

# Copy the content from the underlying image into the newly created volume
# when the container is created instead of when it is started. If false,
# the container engine will not copy the content until the container is started.
# Setting it to true may have negative performance implications.
#
#prepare_volume_on_create = false

# Path to the seccomp.json profile which is used as the default seccomp profile
# for the runtime.
#
#seccomp_profile = "/usr/share/containers/seccomp.json"

# Size of /dev/shm. Specified as <number><unit>.
# Unit is optional, values:
# b (bytes), k (kilobytes), m (megabytes), or g (gigabytes).
# If the unit is omitted, the system uses bytes.
#
#shm_size = "65536k"

# Set timezone in container. Takes IANA timezones as well as "local",
# which sets the timezone in the container to match the host machine.
#
#tz = ""

# Set umask inside the container
#
#umask = "0022"

# Default way to to create a User namespace for the container
# Options are:

#userns = "host"

# Number of UIDs to allocate for the automatic container creation.
# UIDs are allocated from the "container" UIDs listed in
# /etc/subuid & /etc/subgid
#
#userns_size = 65536

# Default way to to create a UTS namespace for the container
# Options are:

#utsns = "private"

# List of volumes. Specified as
# "<directory-on-host>:<directory-in-container>:<options>", for example:
# "/db:/var/lib/db:ro".
# If it is empty or commented out, no volumes will be added
#
#volumes = []

[secrets]
#driver = "file"

[secrets.opts]
#root = "/example/directory"

[network]

# Network backend determines what network driver will be used to set up and tear down container networks.
# Valid values are "cni" and "netavark".
# The default value is empty which means that it will automatically choose CNI or netavark. If there are
# already containers/images or CNI networks preset it will choose CNI.
#
# Before changing this value all containers must be stopped otherwise it is likely that
# iptables rules and network interfaces might leak on the host. A reboot will fix this.
#
#network_backend = ""

# Path to directory where CNI plugin binaries are located.
#
#cni_plugin_dirs = [
#  "/usr/local/libexec/cni",
#  "/usr/libexec/cni",
#  "/usr/local/lib/cni",
#  "/usr/lib/cni",
#  "/opt/cni/bin",
#]

# The network name of the default network to attach pods to.
#
#default_network = "podman"

# The default subnet for the default network given in default_network.
# If a network with that name does not exist, a new network using that name and
# this subnet will be created.
# Must be a valid IPv4 CIDR prefix.
#
#default_subnet = "10.88.0.0/16"

# DefaultSubnetPools is a list of subnets and size which are used to
# allocate subnets automatically for podman network create.
# It will iterate through the list and will pick the first free subnet
# with the given size. This is only used for ipv4 subnets, ipv6 subnets
# are always assigned randomly.
#
#default_subnet_pools = [
#  {"base" = "10.89.0.0/16", "size" = 24},
#  {"base" = "10.90.0.0/15", "size" = 24},
#  {"base" = "10.92.0.0/14", "size" = 24},
#  {"base" = "10.96.0.0/11", "size" = 24},
#  {"base" = "10.128.0.0/9", "size" = 24},
#]

# Path to the directory where network configuration files are located.
# For the CNI backend the default is "/etc/cni/net.d" as root
# and "$HOME/.config/cni/net.d" as rootless.
# For the netavark backend "/etc/containers/networks" is used as root
# and "$graphroot/networks" as rootless.
#
#network_config_dir = "/etc/cni/net.d/"

# Port to use for dns forwarding daemon with netavark in rootful bridge
# mode and dns enabled.
# Using an alternate port might be useful if other dns services should
# run on the machine.
#
#dns_bind_port = 53

[engine]
# Index to the active service
#
#active_service = production

# The compression format to use when pushing an image.

#compression_format = "gzip"


# Cgroup management implementation used for the runtime.
# Valid options "systemd" or "cgroupfs"
#
#cgroup_manager = "systemd"

# Environment variables to pass into conmon
#
#conmon_env_vars = [
#  "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
#]

# Paths to look for the conmon container manager binary
#
#conmon_path = [
#  "/usr/libexec/podman/conmon",
#  "/usr/local/libexec/podman/conmon",
#  "/usr/local/lib/podman/conmon",
#  "/usr/bin/conmon",
#  "/usr/sbin/conmon",
#  "/usr/local/bin/conmon",
#  "/usr/local/sbin/conmon"
#]

# Enforces using docker.io for completing short names in Podman's compatibility
# REST API. Note that this will ignore unqualified-search-registries and
# short-name aliases defined in containers-registries.conf(5).
#compat_api_enforce_docker_hub = true

# Specify the keys sequence used to detach a container.
# Format is a single character [a-Z] or a comma separated sequence of

#detach_keys = "ctrl-p,ctrl-q"

# Determines whether engine will reserve ports on the host when they are
# forwarded to containers. When enabled, when ports are forwarded to containers,
# ports are held open by as long as the container is running, ensuring that
# they cannot be reused by other programs on the host. However, this can cause
# significant memory usage if a container has many ports forwarded to it.
# Disabling this can save memory.
#
#enable_port_reservation = true

# Environment variables to be used when running the container engine (e.g., Podman, Buildah).
# For example "http_proxy=internal.proxy.company.com".
# Note these environment variables will not be used within the container.
# Set the env section under [containers] table, if you want to set environment variables for the container.
#
#env = []

# Define where event logs will be stored, when events_logger is "file".
#events_logfile_path=""


#events_logfile_max_size = "1m"

#events_logger = "journald"

# A is a list of directories which are used to search for helper binaries.
#
#helper_binaries_dir = [
#  "/usr/local/libexec/podman",
#  "/usr/local/lib/podman",
#  "/usr/libexec/podman",
#  "/usr/lib/podman",
#]

# Path to OCI hooks directories for automatically executed hooks.
#
#hooks_dir = [
#  "/usr/share/containers/oci/hooks.d",
#]

# Manifest Type (oci, v2s2, or v2s1) to use when pulling, pushing, building
# container images. By default image pulled and pushed match the format of the
# source image. Building/committing defaults to OCI.
#
#image_default_format = ""

# Default transport method for pulling and pushing for images
#
#image_default_transport = "docker://"

# Maximum number of image layers to be copied (pulled/pushed) simultaneously.
# Not setting this field, or setting it to zero, will fall back to containers/image defaults.
#
#image_parallel_copies = 0

# Tells container engines how to handle the builtin image volumes.
#   * bind: An anonymous named volume will be created and mounted
#     into the container.
#   * tmpfs: The volume is mounted onto the container as a tmpfs,
#     which allows users to create content that disappears when
#     the container is stopped.
#   * ignore: All volumes are just ignored and no action is taken.
#
#image_volume_mode = ""

# Default command to run the infra container
#
#infra_command = "/pause"


#infra_image = ""

# Specify the locking mechanism to use; valid values are "shm" and "file".
# Change the default only if you are sure of what you are doing, in general
# "file" is useful only on platforms where cgo is not available for using the
# faster "shm" lock type. You may need to run "podman system renumber" after
# you change the lock type.
#
#lock_type** = "shm"

# MultiImageArchive - if true, the container engine allows for storing archives
# (e.g., of the docker-archive transport) with multiple images.  By default,
# Podman creates single-image archives.
#
#multi_image_archive = "false"

# Default engine namespace
# If engine is joined to a namespace, it will see only containers and pods
# that were created in the same namespace, and will create new containers and
# pods in that namespace.
# The default namespace is "", which corresponds to no namespace. When no
# namespace is set, all containers and pods are visible.
#
#namespace = ""

# Path to the slirp4netns binary
#
#network_cmd_path = ""


#network_cmd_options = []

# Whether to use chroot instead of pivot_root in the runtime
#
#no_pivot_root = false

# Number of locks available for containers and pods.
# If this is changed, a lock renumber must be performed (e.g. with the
# 'podman system renumber' command).
#
#num_locks = 2048

# Set the exit policy of the pod when the last container exits.
#pod_exit_policy = "continue"

# Whether to pull new image before running a container
#
#pull_policy = "missing"


#remote = false

# Default OCI runtime
#
#runtime = "crun"

# List of the OCI runtimes that support --format=json. When json is supported
# engine will use it for reporting nicer errors.
#
#runtime_supports_json = ["crun", "runc", "kata", "runsc", "krun"]

# List of the OCI runtimes that supports running containers with KVM Separation.
#
#runtime_supports_kvm = ["kata", "krun"]

# List of the OCI runtimes that supports running containers without cgroups.
#
#runtime_supports_nocgroups = ["crun", "krun"]

# Default location for storing temporary container image content. Can be overridden with the TMPDIR environment
# variable. If you specify "storage", then the location of the
# container/storage tmp directory will be used.
# image_copy_tmp_dir="/var/tmp"
# image_copy_tmp_dir="/octelium/podman/tmp-cp"


#service_timeout = 5

# Directory for persistent engine files (database, etc)
# By default, this will be configured relative to where the containers/storage
# stores containers
# Uncomment to change location from this default
#
#static_dir = "/var/lib/containers/storage/libpod"
static_dir = "/octelium/podman/libpod"

# Number of seconds to wait for container to exit before sending kill signal.
#
#stop_timeout = 10

# Number of seconds to wait before exit command in API process is given to.
# This mimics Docker's exec cleanup behaviour, where the default is 5 minutes (value is in seconds).
#
#exit_command_delay = 300

# map of service destinations
#
#[service_destinations]
#  [service_destinations.production]
#     URI to access the Podman service
#     Examples:
#       rootless "unix://run/user/$UID/podman/podman.sock" (Default)
#       rootful "unix://run/podman/podman.sock (Default)
#       remote rootless ssh://engineering.lab.company.com/run/user/1000/podman/podman.sock
#       remote rootful ssh://root@10.10.1.136:22/run/podman/podman.sock
#
#    uri = "ssh://user@production.example.com/run/user/1001/podman/podman.sock"
#    Path to file containing ssh identity key
#    identity = "~/.ssh/id_rsa"

# Directory for temporary files. Must be tmpfs (wiped after reboot)
#
#tmp_dir = "/run/libpod"
# NOT TMPFS
#tmp_dir = "/octelium/podman/tmp-dir"

# Directory for libpod named volumes.
# By default, this will be configured relative to where containers/storage
# stores containers.
# Uncomment to change location from this default.
#
#volume_path = "/var/lib/containers/storage/volumes"

# Default timeout (in seconds) for volume plugin operations.
# Plugins are external programs accessed via a REST API; this sets a timeout
# for requests to that API.
# A value of 0 is treated as no timeout.
#volume_plugin_timeout = 5

# Paths to look for a valid OCI runtime (crun, runc, kata, runsc, krun, etc)
[engine.runtimes]
#crun = [
#  "/usr/bin/crun",
#  "/usr/sbin/crun",
#  "/usr/local/bin/crun",
#  "/usr/local/sbin/crun",
#  "/sbin/crun",
#  "/bin/crun",
#  "/run/current-system/sw/bin/crun",
#]

#kata = [
#  "/usr/bin/kata-runtime",
#  "/usr/sbin/kata-runtime",
#  "/usr/local/bin/kata-runtime",
#  "/usr/local/sbin/kata-runtime",
#  "/sbin/kata-runtime",
#  "/bin/kata-runtime",
#  "/usr/bin/kata-qemu",
#  "/usr/bin/kata-fc",
#]

#runc = [
#  "/usr/bin/runc",
#  "/usr/sbin/runc",
#  "/usr/local/bin/runc",
#  "/usr/local/sbin/runc",
#  "/sbin/runc",
#  "/bin/runc",
#  "/usr/lib/cri-o-runc/sbin/runc",
#]

#runsc = [
#  "/usr/bin/runsc",
#  "/usr/sbin/runsc",
#  "/usr/local/bin/runsc",
#  "/usr/local/sbin/runsc",
#  "/bin/runsc",
#  "/sbin/runsc",
#  "/run/current-system/sw/bin/runsc",
#]

#krun = [
#  "/usr/bin/krun",
#  "/usr/local/bin/krun",
#]

[engine.volume_plugins]
#testplugin = "/run/podman/plugins/test.sock"

[machine]
# Number of CPU's a machine is created with.
#
#cpus=1

# The size of the disk in GB created when init-ing a podman-machine VM.
#
#disk_size=10


# image = "testing"

# Memory in MB a machine is created with.
#
#memory=2048

# The username to use and create on the podman machine OS for rootless
# container access.
#
#user = "core"


# volumes = [
#  "$HOME:$HOME",
#]

# The [machine] table MUST be the last entry in this file.
# (Unless another table is added)
# TOML does not provide a way to end a table other than a further table being
# defined, so every key hereafter will be part of [machine] and not the
# main config.
`

const podmanConfRegistries = `
# For more information on this configuration file, see containers-registries.conf(5).
#
# # An array of host[:port] registries to try when pulling an unqualified image, in order.
unqualified-search-registries = ["docker.io"]
#
# [[registry]]
# # The "prefix" field is used to choose the relevant [[registry]] TOML table;
# # (only) the TOML table with the longest match for the input image name
# # (taking into account namespace/repo/tag/digest separators) is used.
# # 
# # The prefix can also be of the form: *.example.com for wildcard subdomain
# # matching.
# #
# # If the prefix field is missing, it defaults to be the same as the "location" field.
# prefix = "example.com/foo"
#
# # If true, unencrypted HTTP as well as TLS connections with untrusted
# # certificates are allowed.
# insecure = false
#
# # If true, pulling images with matching names is forbidden.
# blocked = false
#
# # The physical location of the "prefix"-rooted namespace.
# #
# # By default, this is equal to "prefix" (in which case "prefix" can be omitted
# # and the [[registry]] TOML table can only specify "location").
# #
# # Example: Given
# #   prefix = "example.com/foo"
# #   location = "internal-registry-for-example.net/bar"
# # requests for the image example.com/foo/myimage:latest will actually work with the
# # internal-registry-for-example.net/bar/myimage:latest image.
#
# # The location can be empty iff prefix is in a
# # wildcarded format: "*.example.com". In this case, the input reference will
# # be used as-is without any rewrite.
# location = internal-registry-for-example.com/bar"
#
# # (Possibly-partial) mirrors for the "prefix"-rooted namespace.
# #
# # The mirrors are attempted in the specified order; the first one that can be
# # contacted and contains the image will be used (and if none of the mirrors contains the image,
# # the primary location specified by the "registry.location" field, or using the unmodified
# # user-specified reference, is tried last).
# #
# # Each TOML table in the "mirror" array can contain the following fields, with the same semantics
# # as if specified in the [[registry]] TOML table directly:
# # - location
# # - insecure
# [[registry.mirror]]
# location = "example-mirror-0.local/mirror-for-foo"
# [[registry.mirror]]
# location = "example-mirror-1.local/mirrors/foo"
# insecure = true
# # Given the above, a pull of example.com/foo/image:latest will try:
# # 1. example-mirror-0.local/mirror-for-foo/image:latest
# # 2. example-mirror-1.local/mirrors/foo/image:latest
# # 3. internal-registry-for-example.net/bar/image:latest
# # in order, and use the first one that exists.
# unqualified-search-registries = ["docker.io"]
`

const podmanConfStorage = `
# This file is the configuration file for all tools
# that use the containers/storage library. The storage.conf file
# overrides all other storage.conf files. Container engines using the
# container/storage library do not inherit fields from other storage.conf
# files.
#
#  Note: The storage.conf file overrides other storage.conf files based on this precedence:
#      /usr/containers/storage.conf
#      /etc/containers/storage.conf
#      $HOME/.config/containers/storage.conf
#      $XDG_CONFIG_HOME/containers/storage.conf (If XDG_CONFIG_HOME is set)
# See man 5 containers-storage.conf for more information
# The "container storage" table contains all of the server options.
[storage]

# Default Storage Driver, Must be set for proper operation.
driver = "overlay"

# Temporary storage location
# runroot = "/run/containers/storage"
runroot = "/octelium/podman/tmp-storage"

# Primary Read/Write location of container storage
# When changing the graphroot location on an SELINUX system, you must
# ensure  the labeling matches the default locations labels with the
# following commands:
# semanage fcontext -a -e /var/lib/containers/storage /NEWSTORAGEPATH
# restorecon -R -v /NEWSTORAGEPATH
# graphroot = "/var/lib/containers/storage"
graphroot = "/octelium/podman/storage"


# Storage path for rootless users
#
# rootless_storage_path = "$HOME/.local/share/containers/storage"
rootless_storage_path = "/octelium/podman/storage-rootless"

[storage.options]
# Storage options to be passed to underlying storage drivers

# AdditionalImageStores is used to pass paths to additional Read/Only image stores
# Must be comma separated list.
additionalimagestores = [
]

# Allows specification of how storage is populated when pulling images. This
# option can speed the pulling process of images compressed with format
# zstd:chunked. Containers/storage looks for files within images that are being
# pulled from a container registry that were previously pulled to the host.  It
# can copy or create a hard link to the existing file when it finds them,
# eliminating the need to pull them from the container registry. These options
# can deduplicate pulling of content, disk storage of content and can allow the
# kernel to use less memory when running containers.

# containers/storage supports four keys
#   * enable_partial_images="true" | "false"
#     Tells containers/storage to look for files previously pulled in storage
#     rather then always pulling them from the container registry.
#   * use_hard_links = "false" | "true"
#     Tells containers/storage to use hard links rather then create new files in
#     the image, if an identical file already existed in storage.
#   * ostree_repos = ""
#     Tells containers/storage where an ostree repository exists that might have
#     previously pulled content which can be used when attempting to avoid
#     pulling content from the container registry
# pull_options = {enable_partial_images = "false", use_hard_links = "false", ostree_repos=""}
# pull_options = {enable_partial_images = "true", use_hard_links = "false", ostree_repos = ""}
# Remap-UIDs/GIDs is the mapping from UIDs/GIDs as they should appear inside of
# a container, to the UIDs/GIDs as they should appear outside of the container,
# and the length of the range of UIDs/GIDs.  Additional mapped sets can be
# listed and will be heeded by libraries, but there are limits to the number of
# mappings which the kernel will allow when you later attempt to run a
# container.
#
# remap-uids = 0:1668442479:65536
# remap-gids = 0:1668442479:65536

# Remap-User/Group is a user name which can be used to look up one or more UID/GID
# ranges in the /etc/subuid or /etc/subgid file.  Mappings are set up starting
# with an in-container ID of 0 and then a host-level ID taken from the lowest
# range that matches the specified name, and using the length of that range.
# Additional ranges are then assigned, using the ranges which specify the
# lowest host-level IDs first, to the lowest not-yet-mapped in-container ID,
# until all of the entries have been used for maps.
#
# remap-user = "containers"
# remap-group = "containers"

# Root-auto-userns-user is a user name which can be used to look up one or more UID/GID
# ranges in the /etc/subuid and /etc/subgid file.  These ranges will be partitioned
# to containers configured to create automatically a user namespace.  Containers
# configured to automatically create a user namespace can still overlap with containers
# having an explicit mapping set.
# This setting is ignored when running as rootless.
# root-auto-userns-user = "storage"
#
# Auto-userns-min-size is the minimum size for a user namespace created automatically.
# auto-userns-min-size=1024
#
# Auto-userns-max-size is the minimum size for a user namespace created automatically.
# auto-userns-max-size=65536

[storage.options.overlay]
# ignore_chown_errors can be set to allow a non privileged user running with
# a single UID within a user namespace to run containers. The user can pull
# and use any image even those with multiple uids.  Note multiple UIDs will be
# squashed down to the default uid in the container.  These images will have no
# separation between the users in the container. Only supported for the overlay
# and vfs drivers.
#ignore_chown_errors = "false"

# Inodes is used to set a maximum inodes of the container image.
# inodes = ""

# Path to an helper program to use for mounting the file system instead of mounting it
# directly.
# mount_program = "/usr/bin/fuse-overlayfs"

# mountopt specifies comma separated list of extra mount options
# mountopt = "nodev,metacopy=on,userxattr"

# Set to skip a PRIVATE bind mount on the storage home directory.
# skip_mount_home = "false"

# Size is used to set a maximum size of the container image.
# size = ""

# ForceMask specifies the permissions mask that is used for new files and
# directories.
#
# The values "shared" and "private" are accepted.
# Octal permission masks are also accepted.
#
#  "": No value specified.
#     All files/directories, get set with the permissions identified within the
#     image.
#  "private": it is equivalent to 0700.
#     All files/directories get set with 0700 permissions.  The owner has rwx
#     access to the files. No other users on the system can access the files.
#     This setting could be used with networked based homedirs.
#  "shared": it is equivalent to 0755.
#     The owner has rwx access to the files and everyone else can read, access
#     and execute them. This setting is useful for sharing containers storage
#     with other users.  For instance have a storage owned by root but shared
#     to rootless users as an additional store.
#     NOTE:  All files within the image are made readable and executable by any
#     user on the system. Even /etc/shadow within your image is now readable by
#     any user.
#
#   OCTAL: Users can experiment with other OCTAL Permissions.
#
#  Note: The force_mask Flag is an experimental feature, it could change in the
#  future.  When "force_mask" is set the original permission mask is stored in
#  the "user.containers.override_stat" xattr and the "mount_program" option must
#  be specified. Mount programs like "/usr/bin/fuse-overlayfs" present the
#  extended attribute permissions to processes within containers rather than the
#  "force_mask"  permissions.
#
# force_mask = ""

[storage.options.thinpool]
# Storage Options for thinpool

# autoextend_percent determines the amount by which pool needs to be
# grown. This is specified in terms of % of pool size. So a value of 20 means
# that when threshold is hit, pool will be grown by 20% of existing
# pool size.
# autoextend_percent = "20"

# autoextend_threshold determines the pool extension threshold in terms
# of percentage of pool size. For example, if threshold is 60, that means when
# pool is 60% full, threshold has been hit.
# autoextend_threshold = "80"

# basesize specifies the size to use when creating the base device, which
# limits the size of images and containers.
# basesize = "10G"

# blocksize specifies a custom blocksize to use for the thin pool.
# blocksize="64k"

# directlvm_device specifies a custom block storage device to use for the
# thin pool. Required if you setup devicemapper.
# directlvm_device = ""

# directlvm_device_force wipes device even if device already has a filesystem.
# directlvm_device_force = "True"

# fs specifies the filesystem type to use for the base device.
# fs="xfs"

# log_level sets the log level of devicemapper.
# 0: LogLevelSuppress 0 (Default)
# 2: LogLevelFatal
# 3: LogLevelErr
# 4: LogLevelWarn
# 5: LogLevelNotice
# 6: LogLevelInfo
# 7: LogLevelDebug
# log_level = "7"

# min_free_space = "10%"

# mkfsarg specifies extra mkfs arguments to be used when creating the base
# device.
# mkfsarg = ""


# metadata_size = ""

# Size is used to set a maximum size of the container image.
# size = ""

# use_deferred_removal marks devicemapper block device for deferred removal.
# If the thinpool is in use when the driver attempts to remove it, the driver
# tells the kernel to remove it as soon as possible. Note this does not free
# up the disk space, use deferred deletion to fully remove the thinpool.
# use_deferred_removal = "True"

# use_deferred_deletion marks thinpool device for deferred deletion.
# If the device is busy when the driver attempts to delete it, the driver
# will attempt to delete device every 30 seconds until successful.
# If the program using the driver exits, the driver will continue attempting
# to cleanup the next time the driver is used. Deferred deletion permanently
# deletes the device and all data stored in device will be lost.
# use_deferred_deletion = "True"

# xfs_nospace_max_retries specifies the maximum number of retries XFS should
# attempt to complete IO when ENOSPC (no space) error is returned by
# underlying storage device.
# xfs_nospace_max_retries = "0"
`
