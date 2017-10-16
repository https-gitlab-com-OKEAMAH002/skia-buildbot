package gce

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	compute "google.golang.org/api/compute/v0.alpha"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	ACCELERATOR_TYPE_NVIDIA_TESLA_K80 = "projects/google.com:skia-buildbots/zones/us-east1-d/acceleratorTypes/nvidia-tesla-k80"

	CPU_PLATFORM_SKYLAKE = "Intel Skylake"

	// Labels can only contain lowercase letters, numbers, underscores, and dashes.
	DATE_FORMAT     = "2006-01-02"
	DATETIME_FORMAT = "2006-01-02_15-04-05"

	DISK_SNAPSHOT_SYSTEMD_PUSHABLE_BASE = "skia-systemd-pushable-base"

	DISK_TYPE_LOCAL_SSD           = "local-ssd"
	DISK_TYPE_PERSISTENT_STANDARD = "pd-standard"
	DISK_TYPE_PERSISTENT_SSD      = "pd-ssd"

	IMAGE_STATUS_READY = "READY"

	MACHINE_TYPE_HIGHMEM_2   = "n1-highmem-2"
	MACHINE_TYPE_HIGHMEM_16  = "n1-highmem-16"
	MACHINE_TYPE_HIGHMEM_32  = "n1-highmem-32"
	MACHINE_TYPE_STANDARD_1  = "n1-standard-1"
	MACHINE_TYPE_STANDARD_2  = "n1-standard-2"
	MACHINE_TYPE_STANDARD_4  = "n1-standard-4"
	MACHINE_TYPE_STANDARD_8  = "n1-standard-8"
	MACHINE_TYPE_STANDARD_16 = "n1-standard-16"
	MACHINE_TYPE_STANDARD_32 = "n1-standard-32"

	MAINTENANCE_POLICY_MIGRATE   = "MIGRATE"
	MAINTENANCE_POLICY_TERMINATE = "TERMINATE"

	NETWORK_DEFAULT = "global/networks/default"

	OS_LINUX   = "Linux"
	OS_WINDOWS = "Windows"

	PROJECT_ID = "google.com:skia-buildbots"

	SERVICE_ACCOUNT_DEFAULT         = "31977622648@project.gserviceaccount.com"
	SERVICE_ACCOUNT_COMPUTE         = "31977622648-compute@developer.gserviceaccount.com"
	SERVICE_ACCOUNT_CHROME_SWARMING = "chrome-swarming-bots@skia-buildbots.google.com.iam.gserviceaccount.com"
	SERVICE_ACCOUNT_CHROMIUM_SWARM  = "chromium-swarm-bots@skia-buildbots.google.com.iam.gserviceaccount.com"

	SETUP_SCRIPT_KEY_LINUX  = "setup-script"
	SETUP_SCRIPT_KEY_WIN    = "sysprep-oobe-script-ps1"
	SETUP_SCRIPT_PATH_LINUX = "/tmp/setup-script.sh"

	USER_CHROME_BOT = "chrome-bot"
	USER_DEFAULT    = "default"

	ZONE_CENTRAL1_B = "us-central1-b"
	ZONE_CENTRAL1_C = "us-central1-c"
	ZONE_EAST1_D    = "us-east1-d"

	ZONE_CT      = ZONE_CENTRAL1_B
	ZONE_DEFAULT = ZONE_CENTRAL1_C
	ZONE_GPU     = ZONE_EAST1_D
	ZONE_SKYLAKE = ZONE_CENTRAL1_B

	diskStatusError = "ERROR"
	diskStatusReady = "READY"

	instanceStatusError   = "ERROR"
	instanceStatusRunning = "RUNNING"
	instanceStatusStopped = "TERMINATED"

	errNotFound      = "\\\"reason\\\": \\\"notFound\\\""
	errAlreadyExists = "\\\"reason\\\": \\\"alreadyExists\\\""

	maxWaitTime = 10 * time.Minute

	winSetupFinishedText   = "Instance setup finished."
	winStartupFinishedText = "Finished running startup scripts."
)

var (
	VALID_OS    = []string{OS_LINUX, OS_WINDOWS}
	VALID_ZONES = []string{ZONE_CENTRAL1_B, ZONE_CENTRAL1_C, ZONE_EAST1_D}
)

// GCloud is a struct used for creating disks and instances in GCE.
type GCloud struct {
	project string
	s       *compute.Service
	workdir string
	zone    string
}

// NewGCloud returns a GCloud instance with a default http client. The
// default client expects a local gcloud_token.data and client_secret.json.
func NewGCloud(zone, workdir string) (*GCloud, error) {
	oauthCacheFile := path.Join(workdir, "gcloud_token.data")
	httpClient, err := auth.NewClient(true, oauthCacheFile, compute.CloudPlatformScope, compute.ComputeScope, compute.DevstorageFullControlScope)
	if err != nil {
		return nil, err
	}
	return NewGCloudWithClient(zone, workdir, httpClient)
}

// NewGCloudWithClient returns a GCloud instance with the specified http client.
func NewGCloudWithClient(zone, workdir string, httpClient *http.Client) (*GCloud, error) {
	s, err := compute.New(httpClient)
	if err != nil {
		return nil, err
	}

	// Verify that we're set up for SSH.
	if _, err := sshArgs(); err != nil {
		return nil, err
	}

	return &GCloud{
		project: PROJECT_ID,
		s:       s,
		workdir: workdir,
		zone:    zone,
	}, nil
}

// Service returns the underlying compute.Service instance.
func (g *GCloud) Service() *compute.Service {
	return g.s
}

// Disk is a struct describing a disk resource in GCE.
type Disk struct {
	// The name of the disk.
	Name string

	// Size of the disk, in gigabytes.
	SizeGb int64

	// Optional, image to flash to the disk. Use only one of SourceImage
	// and SourceSnapshot.
	SourceImage string

	// Optional, snapshot to flash to the disk. Use only one of SourceImage
	// and SourceSnapshot.
	SourceSnapshot string

	// Type of disk, eg. "pd-standard" or "pd-ssd".
	Type string

	// Output only, which instances are using this disk, if any.
	InUseBy []string

	// Optional mountpoint. Default: /mnt/pd0 (see format_and_mount.sh)
	MountPath string
}

// CreateDisk creates the given disk.
func (g *GCloud) CreateDisk(disk *Disk, ignoreExists bool) error {
	sklog.Infof("Creating disk %q", disk.Name)
	d := &compute.Disk{
		Name:   disk.Name,
		SizeGb: disk.SizeGb,
		Type:   fmt.Sprintf("zones/%s/diskTypes/%s", g.zone, disk.Type),
	}
	if disk.SourceImage != "" && disk.SourceSnapshot != "" {
		return fmt.Errorf("Only one of SourceImage and SourceSnapshot may be used.")
	}
	if disk.SourceImage != "" {
		if len(strings.Split(disk.SourceImage, "/")) == 5 {
			d.SourceImage = disk.SourceImage
		} else {
			d.SourceImage = fmt.Sprintf("projects/%s/global/images/%s", g.project, disk.SourceImage)
		}
	} else if disk.SourceSnapshot != "" {
		if len(strings.Split(disk.SourceSnapshot, "/")) == 5 {
			d.SourceSnapshot = disk.SourceSnapshot
		} else {
			d.SourceSnapshot = fmt.Sprintf("projects/%s/global/snapshots/%s", g.project, disk.SourceSnapshot)
		}
	}
	op, err := g.s.Disks.Insert(g.project, g.zone, d).Do()
	if err != nil {
		if strings.Contains(err.Error(), errAlreadyExists) {
			if ignoreExists {
				sklog.Infof("Disk %q already exists; ignoring.", disk.Name)
			} else {
				return fmt.Errorf("Disk %q already exists.", disk.Name)
			}
		} else {
			return err
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to insert disk: %v", op.Error)
	} else {
		if err := g.waitForDisk(disk.Name, diskStatusReady, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully created disk %s", disk.Name)
	}
	return nil
}

// DeleteDisk deletes the given disk.
func (g *GCloud) DeleteDisk(name string, ignoreNotExists bool) error {
	sklog.Infof("Deleting disk %q", name)
	op, err := g.s.Disks.Delete(g.project, g.zone, name).Do()
	if err != nil {
		if strings.Contains(err.Error(), errNotFound) {
			if ignoreNotExists {
				sklog.Infof("Disk %q does not exist; ignoring.", name)
			} else {
				return fmt.Errorf("Disk %q already exists.", name)
			}
		} else {
			return fmt.Errorf("Failed to delete disk %q: %s", name, err)
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to delete disk: %v", op.Error)
	} else {
		if err := g.waitForDisk(name, diskStatusError, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully deleted disk %s", name)
	}
	return nil
}

// ListDisks returns a list of Disks in the project.
func (g *GCloud) ListDisks() ([]*Disk, error) {
	disks := []*Disk{}
	call := g.s.Disks.List(g.project, g.zone)
	if err := call.Pages(context.Background(), func(list *compute.DiskList) error {
		for _, d := range list.Items {
			disk := &Disk{
				Name:   d.Name,
				SizeGb: d.SizeGb,
			}
			if d.SourceImage != "" {
				split := strings.Split(d.SourceImage, "/")
				if len(split) == 5 && split[1] != g.project {
					disk.SourceImage = d.SourceImage
				} else {
					disk.SourceImage = split[len(split)-1]
				}
			}
			if d.SourceSnapshot != "" {
				split := strings.Split(d.SourceSnapshot, "/")
				if len(split) == 5 && split[1] != g.project {
					disk.SourceImage = d.SourceImage
				} else {
					disk.SourceImage = split[len(split)-1]
				}
			}
			if d.Type != "" {
				split := strings.Split(d.Type, "/")
				disk.Type = split[len(split)-1]
			}
			inUseBy := make([]string, 0, len(d.Users))
			for _, user := range d.Users {
				split := strings.Split(user, "/")
				inUseBy = append(inUseBy, split[len(split)-1])
			}
			disk.InUseBy = inUseBy
			disks = append(disks, disk)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return disks, nil
}

// getDiskStatus returns the current status of the disk.
func (g *GCloud) getDiskStatus(name string) string {
	d, err := g.s.Disks.Get(g.project, g.zone, name).Do()
	if err != nil {
		return diskStatusError
	}
	return d.Status
}

// waitForDisk waits until the disk has the given status.
func (g *GCloud) waitForDisk(name, status string, timeout time.Duration) error {
	start := time.Now()
	for st := g.getDiskStatus(name); st != status; st = g.getDiskStatus(name) {
		if time.Now().Sub(start) > timeout {
			return fmt.Errorf("Exceeded timeout of %s", timeout)
		}
		sklog.Infof("Waiting for disk %q (status %s)", name, st)
		time.Sleep(5 * time.Second)
	}
	return nil
}

// Instance is a struct representing a GCE VM instance.
type Instance struct {
	// Information about the boot disk. Required.
	BootDisk *Disk

	// Information about an extra data disk. Optional.
	DataDisks []*Disk

	// External IP address for the instance. Required.
	ExternalIpAddress string

	// Whether or not to include an NVIDIA Tesla k80 GPU on the instance.
	Gpu bool

	// Files to download from Google Storage.
	GSDownloads []*GSDownload

	// GCE machine type specification, eg. "n1-standard-16".
	MachineType string

	// Maintenance policy. Default is MAINTENANCE_POLICY_MIGRATE, which is
	// not supported for preemtible instances.
	MaintenancePolicy string

	// Instance-level metadata keys and values.
	Metadata map[string]string

	// Files to create based on metadata. Map keys are destination paths on
	// the GCE instance and values are the source URLs (see
	// metadata.METADATA_URL). Paths May be absolute or relative (to the
	// default user's home dir, eg. /home/default).
	MetadataDownloads map[string]string

	// Minimum CPU platform, eg. CPU_PLATFORM_SKYLAKE.  Default is
	// determined by GCE.
	MinCpuPlatform string

	// Name of the instance.
	Name string

	// Operating system of the instance.
	Os string

	// Password is the default user's password. Only used for Windows.
	Password string

	// Auth scopes for the instance.
	Scopes []string

	// Path to a setup script for the instance, optional. Should be either
	// absolute or relative to the parent GCloud instance's workdir. The
	// setup script runs once after the instance is created. For Windows,
	// this is assumed to be a PowerShell script and runs during sysprep.
	// For Linux, the script needs to be executable via the shell (ie. use
	// a shebang for Python scripts).
	SetupScript string

	// The service account to use for this instance. Will default to
	// SERVICE_ACCOUNT_DEFAULT if unspecified.
	ServiceAccount string

	// Path to a startup script for the instance, optional. Should be either
	// absolute or relative to the parent GCloud instance's workdir. The
	// startup script runs as root every time the instance starts up. For
	// Windows, this is assumed to be a PowerShell script. For Linux, the
	// script needs to be executable via the shell (ie. use a shebang for
	// Python scripts).
	StartupScript string

	// Tags for the instance.
	Tags []string

	// Default user name for the instance.
	User string
}

// scriptToMetadata reads the given script and inserts it into the Instance's
// metadata.
func scriptToMetadata(vm *Instance, key, path string) error {
	var script string
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	script = string(b)
	if vm.Os == OS_WINDOWS {
		script = util.ToDos(script)
	}
	if vm.Metadata == nil {
		vm.Metadata = map[string]string{}
	}
	vm.Metadata[key] = script
	return nil
}

// setupScriptToMetadata reads the setup script and returns a MetadataItems.
func setupScriptToMetadata(vm *Instance) error {
	key := SETUP_SCRIPT_KEY_WIN
	if vm.Os != OS_WINDOWS {
		key = SETUP_SCRIPT_KEY_LINUX
		if vm.MetadataDownloads == nil {
			vm.MetadataDownloads = map[string]string{}
		}
		vm.MetadataDownloads[SETUP_SCRIPT_PATH_LINUX] = fmt.Sprintf(metadata.METADATA_URL, "instance", SETUP_SCRIPT_KEY_LINUX)
	}
	return scriptToMetadata(vm, key, vm.SetupScript)
}

// startupScriptToMetadata reads the startup script and returns a MetadataItems.
func startupScriptToMetadata(vm *Instance) error {
	key := "startup-script"
	if vm.Os == OS_WINDOWS {
		key = "windows-startup-script-ps1"
	}
	return scriptToMetadata(vm, key, vm.StartupScript)
}

// validateDataDisks validates the data disk definitions.
func (g *GCloud) validateDataDisks(vm *Instance) error {
	// Go through the data disks and make sure they are consistent.
	allDiskNames := util.StringSet{}
	allMountPaths := util.StringSet{}
	for _, dataDisk := range vm.DataDisks {
		// Make sure the name is not empty and unique.
		if (dataDisk.Name == "") || (allDiskNames[dataDisk.Name]) {
			return fmt.Errorf("Data disk name is '%s'. It cannot be empty and must be unique for this instance.", dataDisk.Name)
		}
		allDiskNames[dataDisk.Name] = true

		// Make sure the mount path are valid and unique.
		if !filepath.IsAbs(dataDisk.MountPath) || allMountPaths[dataDisk.MountPath] {
			return fmt.Errorf("Mount path %s is either not a unique path or not absolute.", dataDisk.MountPath)
		}
		allMountPaths[dataDisk.MountPath] = true

		if vm.Os == OS_WINDOWS {
			return fmt.Errorf("Data disks are not currently supported on Windows.")
		}
	}
	return nil
}

// createDataDisks creates the defined data disks with the exception of local SSDs.
func (g *GCloud) createDataDisks(vm *Instance, ignoreExists bool) error {
	for _, dataDisk := range vm.DataDisks {
		// Local SSDs are created with the instance.
		if dataDisk.Type != DISK_TYPE_LOCAL_SSD {
			if err := g.CreateDisk(dataDisk, ignoreExists); err != nil {
				return err
			}
		}
	}

	return nil
}

// createInstance creates the given VM instance.
func (g *GCloud) createInstance(vm *Instance, ignoreExists bool) error {
	sklog.Infof("Creating instance %q", vm.Name)
	if vm.Name == "" {
		return fmt.Errorf("Instance name is required.")
	}
	if vm.Os == "" {
		return fmt.Errorf("Instance OS is required.")
	}

	disks := []*compute.AttachedDisk{}
	if vm.BootDisk != nil {
		disks = append(disks, &compute.AttachedDisk{
			AutoDelete: true,
			Boot:       true,
			DeviceName: vm.BootDisk.Name,
			Source:     fmt.Sprintf("projects/%s/zones/%s/disks/%s", g.project, g.zone, vm.BootDisk.Name),
		})
	}

	for _, dataDisk := range vm.DataDisks {
		d := &compute.AttachedDisk{
			DeviceName: dataDisk.Name,
		}
		if dataDisk.Type == DISK_TYPE_LOCAL_SSD {
			// In this case, we didn't create the disk beforehand.
			d.AutoDelete = true
			d.InitializeParams = &compute.AttachedDiskInitializeParams{
				DiskType: fmt.Sprintf("zones/%s/diskTypes/%s", g.zone, dataDisk.Type),
			}
			d.Type = "SCRATCH"
		} else {
			d.Source = fmt.Sprintf("projects/%s/zones/%s/disks/%s", g.project, g.zone, dataDisk.Name)
		}
		disks = append(disks, d)
	}

	if vm.Os == OS_WINDOWS && vm.User != "" && vm.Password != "" {
		if vm.Metadata == nil {
			vm.Metadata = map[string]string{}
		}
		vm.Metadata["gce-initial-windows-user"] = vm.User
		vm.Metadata["gce-initial-windows-password"] = vm.Password
	}
	if vm.MaintenancePolicy == "" {
		vm.MaintenancePolicy = MAINTENANCE_POLICY_MIGRATE
	}
	if vm.SetupScript != "" {
		if err := setupScriptToMetadata(vm); err != nil {
			return err
		}
	}
	if vm.ServiceAccount == "" {
		vm.ServiceAccount = SERVICE_ACCOUNT_DEFAULT
	}
	if vm.Os == OS_WINDOWS && vm.StartupScript != "" {
		// On Windows, the setup script runs automatically during
		// sysprep which is before the startup script runs. On Linux
		// the startup script does not run automatically, so to ensure
		// that the startup script runs after the setup script, we have
		// to wait to set the startup-script metadata item until after
		// we have manually run the setup script.
		if err := startupScriptToMetadata(vm); err != nil {
			return err
		}
	}
	metadata := make([]*compute.MetadataItems, 0, len(vm.Metadata))
	for k, v := range vm.Metadata {
		metadata = append(metadata, &compute.MetadataItems{
			Key:   k,
			Value: v,
		})
	}
	i := &compute.Instance{
		Disks:       disks,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", g.zone, vm.MachineType),
		Metadata: &compute.Metadata{
			Items: metadata,
		},
		MinCpuPlatform: vm.MinCpuPlatform,
		Name:           vm.Name,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: []*compute.AccessConfig{
					{
						NatIP: vm.ExternalIpAddress,
						Type:  "ONE_TO_ONE_NAT",
					},
				},
				Network: NETWORK_DEFAULT,
			},
		},
		Scheduling: &compute.Scheduling{
			OnHostMaintenance: vm.MaintenancePolicy,
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email:  vm.ServiceAccount,
				Scopes: vm.Scopes,
			},
		},
		Tags: &compute.Tags{
			Items: vm.Tags,
		},
	}
	if vm.Gpu {
		i.GuestAccelerators = []*compute.AcceleratorConfig{
			&compute.AcceleratorConfig{
				AcceleratorCount: 1,
				AcceleratorType:  ACCELERATOR_TYPE_NVIDIA_TESLA_K80,
			},
		}
	}
	op, err := g.s.Instances.Insert(g.project, g.zone, i).Do()
	if err != nil {
		if strings.Contains(err.Error(), errAlreadyExists) {
			if ignoreExists {
				sklog.Infof("Instance %q already exists; ignoring.", vm.Name)
			} else {
				return fmt.Errorf("Instance %q already exists.", vm.Name)
			}
		} else {
			return err
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to insert instance: %v", op.Error)
	} else {
		if err := g.waitForInstance(vm.Name, instanceStatusRunning, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully created instance %s", vm.Name)
	}
	// Obtain the instance IP address if necessary.
	if vm.ExternalIpAddress == "" {
		ip, err := g.GetIpAddress(vm)
		if err != nil {
			return err
		}
		vm.ExternalIpAddress = ip
	}
	if err := g.WaitForInstanceReady(vm, maxWaitTime); err != nil {
		return err
	}
	return nil
}

// DeleteInstance deletes the given GCE VM instance.
func (g *GCloud) DeleteInstance(name string, ignoreNotExists bool) error {
	sklog.Infof("Deleting instance %q", name)
	op, err := g.s.Instances.Delete(g.project, g.zone, name).Do()
	if err != nil {
		if strings.Contains(err.Error(), errNotFound) {
			if ignoreNotExists {
				sklog.Infof("Instance %q does not exist; ignoring.", name)
			} else {
				return fmt.Errorf("Instance %q does not exist.", name)
			}
		} else {
			return fmt.Errorf("Failed to delete instance %q: %s", name, err)
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to delete instance: %v", op.Error)
	} else {
		if err := g.waitForInstance(name, instanceStatusError, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully deleted instance %s", name)
	}
	return nil
}

// IsInstanceRunning returns whether the instance is in running state.
func (g *GCloud) IsInstanceRunning(name string) bool {
	return g.getInstanceStatus(name) == instanceStatusRunning
}

// getInstanceStatus returns the current status of the instance.
func (g *GCloud) getInstanceStatus(name string) string {
	i, err := g.s.Instances.Get(g.project, g.zone, name).Do()
	if err != nil {
		return instanceStatusError
	}
	return i.Status
}

// waitForInstance waits until the instance has the given status.
func (g *GCloud) waitForInstance(name, status string, timeout time.Duration) error {
	start := time.Now()
	for st := g.getInstanceStatus(name); st != status; st = g.getInstanceStatus(name) {
		if time.Now().Sub(start) > timeout {
			return fmt.Errorf("Instance did not have status %q within timeout of %s", status, timeout)
		}
		sklog.Infof("Waiting for instance %q (status %s)", name, st)
		time.Sleep(5 * time.Second)
	}
	return nil
}

// GetIpAddress obtains the IP address for the Instance.
func (g *GCloud) GetIpAddress(vm *Instance) (string, error) {
	inst, err := g.s.Instances.Get(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return "", err
	}
	if len(inst.NetworkInterfaces) != 1 {
		return "", fmt.Errorf("Failed to obtain IP address: Instance has incorrect number of network interfaces: %d", len(inst.NetworkInterfaces))
	}
	if len(inst.NetworkInterfaces[0].AccessConfigs) != 1 {
		return "", fmt.Errorf("Failed to obtain IP address: Instance network interface has incorrect number of access configs: %d", len(inst.NetworkInterfaces[0].AccessConfigs))
	}
	ip := inst.NetworkInterfaces[0].AccessConfigs[0].NatIP
	if ip == "" {
		return "", fmt.Errorf("Failed to obtain IP address: Got empty IP address.")
	}
	return ip, nil
}

// sshArgs returns options for SSH or an error if applicable.
func sshArgs() ([]string, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	keyFile := path.Join(usr.HomeDir, ".ssh", "google_compute_engine")
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("You need to create an SSH key at %s, per https://cloud.google.com/compute/docs/instances/connecting-to-instance#generatesshkeypair", keyFile)
	}
	return []string{
		"-q", "-i", keyFile,
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
	}, nil
}

// Ssh logs into the instance and runs the given command. Returns any output
// and an error if applicable.
func (g *GCloud) Ssh(vm *Instance, cmd ...string) (string, error) {
	if vm.Os == OS_WINDOWS {
		return "", fmt.Errorf("Cannot SSH into Windows machines (for: %v)", cmd)
	}
	if vm.ExternalIpAddress == "" {
		ip, err := g.GetIpAddress(vm)
		if err != nil {
			return "", err
		}
		vm.ExternalIpAddress = ip
	}
	args, err := sshArgs()
	if err != nil {
		return "", err
	}
	command := []string{"ssh"}
	command = append(command, args...)
	command = append(command, fmt.Sprintf("%s@%s", vm.User, vm.ExternalIpAddress))
	command = append(command, cmd...)
	return exec.RunCwd(".", command...)
}

// Scp copies files to the instance. The src argument is expected to be
// absolute.
func (g *GCloud) Scp(vm *Instance, src, dst string) error {
	if vm.Os == OS_WINDOWS {
		return fmt.Errorf("Cannot SCP to Windows machines (for: %s)", dst)
	}
	if vm.ExternalIpAddress == "" {
		ip, err := g.GetIpAddress(vm)
		if err != nil {
			return err
		}
		vm.ExternalIpAddress = ip
	}
	if !filepath.IsAbs(src) {
		return fmt.Errorf("%q is not an absolute path.", src)
	}
	args, err := sshArgs()
	if err != nil {
		return err
	}
	command := []string{"scp"}
	command = append(command, args...)
	command = append(command, src, fmt.Sprintf("%s@%s:%s", vm.User, vm.ExternalIpAddress, dst))
	sklog.Infof("Copying %s -> %s@%s:%s", src, vm.User, vm.Name, dst)
	_, err = exec.RunCwd(".", command...)
	return err
}

// Stop stops the instance and returns when the operation completes.
func (g *GCloud) Stop(vm *Instance) error {
	op, err := g.s.Instances.Stop(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return err
	} else if op.Error != nil {
		return fmt.Errorf("Failed to stop instance: %v", op.Error)
	}
	return g.waitForInstance(vm.Name, instanceStatusStopped, maxWaitTime)
}

// StartWithoutReadyCheck starts the instance and returns when the instance is in RUNNING state.
// Note: This method does not wait for the instance to be ready (ssh-able).
func (g *GCloud) StartWithoutReadyCheck(vm *Instance) error {
	op, err := g.s.Instances.Start(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return err
	} else if op.Error != nil {
		return fmt.Errorf("Failed to start instance: %v", op.Error)
	}
	if err := g.waitForInstance(vm.Name, instanceStatusRunning, maxWaitTime); err != nil {
		return err
	}

	// Instance IP address may change at reboot.
	ip, err := g.GetIpAddress(vm)
	if err != nil {
		return err
	}
	vm.ExternalIpAddress = ip

	return nil
}

// Stop stops the instance and returns when the instance is ready (ssh-able).
func (g *GCloud) Start(vm *Instance) error {
	if err := g.StartWithoutReadyCheck(vm); err != nil {
		return err
	}

	return g.WaitForInstanceReady(vm, maxWaitTime)
}

// Reboot stops and starts the instance. Returns when the instance is ready to use.
func (g *GCloud) Reboot(vm *Instance) error {
	sklog.Infof("Rebooting instance %q", vm.Name)
	if err := g.Stop(vm); err != nil {
		return err
	}
	return g.Start(vm)
}

// IsInstanceReady returns true iff the instance is ready.
func (g *GCloud) IsInstanceReady(vm *Instance) (bool, error) {
	if vm.Os == OS_WINDOWS {
		serial, err := g.s.Instances.GetSerialPortOutput(g.project, g.zone, vm.Name).Do()
		if err != nil {
			return false, err
		}
		if strings.Contains(serial.Contents, winStartupFinishedText) {
			return true, nil
		}
		if strings.Contains(serial.Contents, winSetupFinishedText) {
			return true, nil
		}
		return false, nil
	} else {
		if _, err := g.Ssh(vm, "true"); err != nil {
			return false, nil
		}
		return true, nil
	}
}

// WaitForInstanceReady waits until the instance is ready to use.
func (g *GCloud) WaitForInstanceReady(vm *Instance, timeout time.Duration) error {
	start := time.Now()
	if err := g.waitForInstance(vm.Name, instanceStatusRunning, timeout); err != nil {
		return err
	}
	for {
		if time.Now().Sub(start) > timeout {
			return fmt.Errorf("Instance was not ready within timeout of %s", timeout)
		}
		ready, err := g.IsInstanceReady(vm)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		sklog.Infof("Waiting for instance %q to be ready.", vm.Name)
		time.Sleep(5 * time.Second)
	}
}

// DownloadFile downloads the given file from Google Cloud Storage to the
// instance.
func (g *GCloud) DownloadFile(vm *Instance, f *GSDownload) error {
	if _, err := g.Ssh(vm, "gsutil", "cp", f.Source, f.Dest); err != nil {
		return err
	}
	if f.Mode != "" {
		if _, err := g.Ssh(vm, "chmod", f.Mode, f.Dest); err != nil {
			return err
		}
	}
	return nil
}

// GetFileFromMetadata downloads the given metadata entry to a file.
func (g *GCloud) GetFileFromMetadata(vm *Instance, url, dst string) error {
	_, err := g.Ssh(vm, "wget", "--header", "'Metadata-Flavor: Google'", "--output-document", dst, url)
	return err
}

// SafeFormatAndMount copies the safe_format_and_mount script to the instance
// and runs it for all data disks.
func (g *GCloud) SafeFormatAndMount(vm *Instance) error {
	// Copy the format_and_mount.sh and safe_format_and_mount
	// scripts to the instance.
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	if err := g.Scp(vm, path.Join(dir, "format_and_mount.sh"), "/tmp/format_and_mount.sh"); err != nil {
		return err
	}
	if err := g.Scp(vm, path.Join(dir, "safe_format_and_mount"), "/tmp/safe_format_and_mount"); err != nil {
		return err
	}

	// Run format_and_mount.sh.
	for _, dataDisk := range vm.DataDisks {
		if _, err := g.Ssh(vm, "/tmp/format_and_mount.sh", dataDisk.Name, dataDisk.MountPath); err != nil {
			if !strings.Contains(err.Error(), "is already mounted") {
				return err
			}
		}
	}

	return nil
}

// SetMetadata sets the given metadata on the instance.
func (g *GCloud) SetMetadata(vm *Instance, md map[string]string) error {
	items := make([]*compute.MetadataItems, 0, len(md))
	for k, v := range md {
		items = append(items, &compute.MetadataItems{
			Key:   k,
			Value: v,
		})
	}
	// Retrieve the existing instance metadata fingerprint, which is
	// required in order to update the metadata.
	inst, err := g.s.Instances.Get(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return fmt.Errorf("Failed to retrieve instance before setting metadata: %s", err)
	}
	m := &compute.Metadata{
		Items: items,
	}
	if inst.Metadata != nil {
		m.Fingerprint = inst.Metadata.Fingerprint
	}
	// Set the metadata.
	op, err := g.s.Instances.SetMetadata(g.project, g.zone, vm.Name, m).Do()
	if err != nil {
		return fmt.Errorf("Failed to set instance metadata: %s", err)
	} else if op.Error != nil {
		return fmt.Errorf("Failed to set instance metadata: %v", op.Error)
	}
	return nil
}

// CreateAndSetup creates an instance and all its disks and performs any
// additional setup steps.
func (g *GCloud) CreateAndSetup(vm *Instance, ignoreExists bool) error {
	// Validate the data disk definitions first.
	if err := g.validateDataDisks(vm); err != nil {
		return err
	}

	// Create the boot disk.
	if vm.BootDisk != nil {
		if err := g.CreateDisk(vm.BootDisk, ignoreExists); err != nil {
			return err
		}
	}

	// Create the data disks.
	if err := g.createDataDisks(vm, ignoreExists); err != nil {
		return err
	}

	// Create the instance.
	if err := g.createInstance(vm, ignoreExists); err != nil {
		return err
	}

	if vm.Os == OS_WINDOWS {
		// Set the metadata on the instance again, due to a bug
		// which is lost to time.
		if err := g.SetMetadata(vm, vm.Metadata); err != nil {
			return err
		}
	} else {
		// There is a setup process which takes place after instance
		// creation. It holds the dpkg lock and it reboots the instance
		// when finished, so we need to wait for it to complete before
		// performing our own setup.
		sklog.Infof("Waiting for setup on %s to complete.", vm.Name)
		if _, err := g.Ssh(vm, "sleep", "300"); err != nil {
			sklog.Infof("Setup finished on %s", vm.Name)
		} else {
			sklog.Infof("Setup did not finish on %s within 5 minutes. Continuing anyway.", vm.Name)
		}

		// Instance IP address may change at reboot.
		ip, err := g.GetIpAddress(vm)
		if err != nil {
			return err
		}
		vm.ExternalIpAddress = ip

		if err := g.WaitForInstanceReady(vm, maxWaitTime); err != nil {
			return err
		}
	}

	// Format and mount all data disks.
	if len(vm.DataDisks) > 0 {
		if err := g.SafeFormatAndMount(vm); err != nil {
			return err
		}
	}

	// GSutil downloads.
	for _, f := range vm.GSDownloads {
		if err := g.DownloadFile(vm, f); err != nil {
			return err
		}
	}

	// Metadata downloads.
	for dst, src := range vm.MetadataDownloads {
		if err := g.GetFileFromMetadata(vm, src, dst); err != nil {
			return err
		}
	}

	// On Windows, the setup script runs automatically during sysprep. On
	// Linux, we have to run the setup script manually. In order to ensure
	// that the setup script runs before the startup script, we delay
	// setting the startup-script in metadata until after we've run the
	// setup script.
	if vm.Os != OS_WINDOWS {
		if vm.SetupScript != "" {
			if _, err := g.Ssh(vm, "sudo", "chmod", "+x", SETUP_SCRIPT_PATH_LINUX, "&&", SETUP_SCRIPT_PATH_LINUX); err != nil {
				return err
			}
		}
		if vm.StartupScript != "" {
			if err := startupScriptToMetadata(vm); err != nil {
				return err
			}
			if err := g.SetMetadata(vm, vm.Metadata); err != nil {
				return err
			}
		}
	}

	// Reboot the instance. On Windows, this will cause the startup script to run.
	if err := g.Reboot(vm); err != nil {
		return err
	}

	return nil
}

// Delete removes the instance and (maybe) its disks.
func (g *GCloud) Delete(vm *Instance, ignoreNotExists, deleteDataDisk bool) error {
	// Delete the instance. The boot disk will be auto-deleted.
	if err := g.DeleteInstance(vm.Name, ignoreNotExists); err != nil {
		return err
	}
	if ignoreNotExists && vm.BootDisk != nil {
		// In case creating the boot disk succeeded but creating the instance
		// failed, with ignoreNotExists, DeleteInstance could succeed without
		// cleaning up the orphaned boot disk. Do that here.
		if err := g.DeleteDisk(vm.BootDisk.Name, ignoreNotExists); err != nil {
			return fmt.Errorf("Failed to delete boot disk %q: %s", vm.BootDisk.Name, err)
		}
	}
	// Only delete the data disk(s) if explicitly told to do so.
	// Local SSDs are auto-deleted with the instance.
	if deleteDataDisk {
		for _, dataDisk := range vm.DataDisks {
			if dataDisk.Type != DISK_TYPE_LOCAL_SSD {
				if err := g.DeleteDisk(dataDisk.Name, ignoreNotExists); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// GetImages returns all of the images from the project.
func (g *GCloud) GetImages() ([]*compute.Image, error) {
	rv := []*compute.Image{}
	page := ""
	for {
		images, err := g.s.Images.List(g.project).PageToken(page).Do()
		if err != nil {
			return nil, fmt.Errorf("Failed to load the list of images: %s", err)
		}
		rv = append(rv, images.Items...)
		if images.NextPageToken == "" {
			return rv, nil
		}
		page = images.NextPageToken
	}
}

// CaptureImage captures an image from the instance's boot disk. The instance
// has to be deleted in order to capture the image, and we delete the boot disk
// after capture for cleanliness.
func (g *GCloud) CaptureImage(vm *Instance, family, description string) error {
	// Create an image name based on the family, current date, and number of
	// images created today.
	images, err := g.GetImages()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	imageName := fmt.Sprintf("%s-v%s", family, now.Format(DATE_FORMAT))
	suffix := 0
	for _, image := range images {
		if strings.HasPrefix(image.Name, imageName) {
			suffix++
		}
	}
	imageName = fmt.Sprintf("%s-%03d", imageName, suffix)
	sklog.Infof("About to capture image %q", imageName)

	// Set auto-delete to off for the boot disk.
	sklog.Infof("Turning off auto-delete for %q", vm.BootDisk.Name)
	op, err := g.s.Instances.SetDiskAutoDelete(g.project, g.zone, vm.Name, false, vm.BootDisk.Name).Do()
	if err != nil {
		return fmt.Errorf("Failed to set auto-delete on disk %q: %s", vm.BootDisk.Name, err)
	} else if op.Error != nil {
		return fmt.Errorf("Failed to set auto-delete on disk %q: %s", vm.BootDisk.Name, op.Error)
	}
	user := strings.Split(op.User, "@")[0]

	// Spin until auto-delete is actually off for the instance.
	started := time.Now()
	for {
		time.Sleep(5 * time.Second)
		inst, err := g.s.Instances.Get(g.project, g.zone, vm.Name).Do()
		if err != nil {
			return fmt.Errorf("Failed to retrieve instance details: %s", err)
		}
		var d *compute.AttachedDisk
		for _, disk := range inst.Disks {
			if disk.Boot {
				d = disk
				break
			}
		}
		if d == nil {
			return fmt.Errorf("Unable to find the boot disk!")
		}
		if !d.AutoDelete {
			break
		}
		if time.Now().Sub(started) > maxWaitTime {
			return fmt.Errorf("Auto-delete was not unset on %q within the acceptable time period.", vm.BootDisk.Name)
		}
		sklog.Infof("Waiting for auto-delete to be off for %q", vm.BootDisk.Name)
	}

	// Delete the instance.
	if err := g.DeleteInstance(vm.Name, true); err != nil {
		return err
	}

	// Capture the image.
	sklog.Infof("Capturing disk image.")
	op, err = g.s.Images.Insert(g.project, &compute.Image{
		Description: description,
		Family:      family,
		Labels: map[string]string{
			"created-by": user,
			"created-on": now.Format(DATETIME_FORMAT),
		},
		Name:       imageName,
		SourceDisk: fmt.Sprintf("projects/%s/zones/%s/disks/%s", g.project, g.zone, vm.BootDisk.Name),
	}).Do()
	if err != nil {
		return fmt.Errorf("Failed to capture image of %q: %s", vm.BootDisk.Name, err)
	} else if op.Error != nil {
		return fmt.Errorf("Failed to capture image of %q: %s", vm.BootDisk.Name, op.Error)
	}
	// Wait for the image capture to complete.
	started = time.Now()
	for {
		time.Sleep(5 * time.Second)
		images, err := g.GetImages()
		if err != nil {
			return err
		}
		found := false
		for _, img := range images {
			if img.Name == imageName && img.Status == IMAGE_STATUS_READY {
				found = true
				break
			}
		}
		if found {
			break
		}
		sklog.Infof("Waiting for image capture to finish.")
	}

	// Delete the boot disk.
	sklog.Infof("Deleting disk %q", vm.BootDisk.Name)
	op, err = g.s.Disks.Delete(g.project, g.zone, vm.BootDisk.Name).Do()
	if err != nil {
		return fmt.Errorf("Failed to delete disk %q: %s", vm.BootDisk.Name, err)
	} else if op.Error != nil {
		return fmt.Errorf("Failed to delete disk %q: %s", vm.BootDisk.Name, op.Error)
	}
	sklog.Infof("Successfully captured image %q", imageName)
	return nil
}

// A file to be downloaded from GS.
type GSDownload struct {
	// Full GS path of the file to download.
	Source string
	// Absolute or relative (to the user's home dir) destination path of the
	// file to download.
	Dest string
	// Mode, as accepted by the chmod command, for the file. If not
	// specified, the default file mode is used.
	Mode string
}
