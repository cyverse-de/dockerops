package dockerops

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"context"

	"github.com/cyverse-de/configurate"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/spf13/viper"
	"gopkg.in/cyverse-de/model.v1"
)

var (
	s   *model.Job
	cfg *viper.Viper
)

func shouldrun() bool {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "" {
		return true
	}
	return false
}

func uri() string {
	return "tcp://dind:2375"
}

func JSONData() ([]byte, error) {
	f, err := os.Open("../test/test_runner.json")
	if err != nil {
		return nil, err
	}
	c, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return c, err
}

func _inittests(t *testing.T, memoize bool) *model.Job {
	var err error
	if s == nil || !memoize {
		cfg, err = configurate.Init("../test/test_config.yaml")
		if err != nil {
			t.Fatal(err)
		}
		cfg.Set("irods.base", "/path/to/irodsbase")
		cfg.Set("irods.host", "hostname")
		cfg.Set("irods.port", "1247")
		cfg.Set("irods.user", "user")
		cfg.Set("irods.pass", "pass")
		cfg.Set("irods.zone", "test")
		cfg.Set("irods.resc", "")
		cfg.Set("condor.log_path", "/path/to/logs")
		cfg.Set("condor.porklock_tag", "test")
		cfg.Set("condor.filter_files", "foo,bar,baz,blippy")
		cfg.Set("condor.request_disk", "0")
		data, err := JSONData()
		if err != nil {
			t.Error(err)
		}
		s, err = model.NewFromData(cfg, data)
		if err != nil {
			t.Error(err)
		}
	}
	return s
}

func inittests(t *testing.T) *model.Job {
	return _inittests(t, true)
}

func TestNewDocker(t *testing.T) {
	if !shouldrun() {
		return
	}
	_, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}
}

func TestIsContainer(t *testing.T) {
	if !shouldrun() {
		return
	}
	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}
	actual, err := dc.IsContainer("test_not_there")
	if err != nil {
		t.Error(err)
	}
	if actual {
		t.Error("IsContainer returned true instead of false")
	}
}

func TestPull(t *testing.T) {
	if !shouldrun() {
		return
	}
	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}
	err = dc.Pull("alpine", "latest")
	if err != nil {
		t.Error(err)
	}
}

func TestCreateIsContainerAndNukeByName(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	err = dc.Pull("alpine", "latest")
	if err != nil {
		t.Error(err)
	}

	// If the container already exists, it could be left over from a previous,
	// test, therefore nuke it.
	exists, err := dc.IsContainer(job.Steps[0].Component.Container.Name)
	if err != nil {
		t.Error(err)
	}
	if exists {
		err = dc.NukeContainerByName(job.Steps[0].Component.Container.Name)
		if err != nil {
			t.Error(err)
		}
	}

	// Create the container we actually want to test against
	containerID, err := dc.CreateContainerFromStep(&job.Steps[0], job.InvocationID)
	if err != nil {
		t.Error(err)
	}
	if containerID == "" {
		t.Error("CreateContainerFromStep created a container with a blank ID")
	}

	containerJSON, err := dc.Client.ContainerInspect(dc.ctx, containerID)
	if err != nil {
		t.Error(err)
	}

	expectedInt := job.Steps[0].Component.Container.MemoryLimit
	actualInt := containerJSON.HostConfig.Memory
	if actualInt != expectedInt {
		t.Errorf("Config.Memory was %d instead of %d\n", actualInt, expectedInt)
	}

	expectedInt = job.Steps[0].Component.Container.CPUShares
	actualInt = containerJSON.HostConfig.CPUShares
	if actualInt != expectedInt {
		t.Errorf("Config.CPUShares was %d instead of %d\n", actualInt, expectedInt)
	}

	expected := job.Steps[0].Component.Container.EntryPoint
	actual := containerJSON.Config.Entrypoint[0]
	if actual != expected {
		t.Errorf("Config.Entrypoint was %s instead of %s\n", actual, expected)
	}

	expected = job.Steps[0].Component.Container.NetworkMode
	actual = string(containerJSON.HostConfig.NetworkMode)
	if actual != expected {
		t.Errorf("HostConfig.NetworkMode was %s instead of %s\n", actual, expected)
	}

	expected = "alpine:latest"
	actual = containerJSON.Config.Image
	if actual != expected {
		t.Errorf("Config.Image was %s instead of %s\n", actual, expected)
	}

	expected = "/work"
	actual = containerJSON.Config.WorkingDir
	if actual != expected {
		t.Errorf("Config.WorkingDir was %s instead of %s\n", actual, expected)
	}

	found := false
	for _, e := range containerJSON.Config.Env {
		if e == "food=banana" {
			found = true
		}
	}
	if !found {
		t.Error("Didn't find 'food=banana' in Config.Env.")
	}

	found = false
	for _, e := range containerJSON.Config.Env {
		if e == "foo=bar" {
			found = true
		}
	}
	if !found {
		t.Error("Didn't find 'foo=bar' in Config.Env.")
	}

	expectedConfig := container.LogConfig{Type: "none"}
	actualConfig := containerJSON.HostConfig.LogConfig
	if expectedConfig.Type != actualConfig.Type {
		t.Errorf("HostConfig.LogConfig was %s instead of %s", actualConfig.Type, expectedConfig.Type)
	}

	expectedList := strslice.StrSlice{"This is a test"}
	actualList := containerJSON.Config.Cmd
	if !reflect.DeepEqual(expectedList, actualList) {
		t.Errorf("Config.Cmd was:\n\t%#v\ninstead of:\n\t%#v\n", actualList, expectedList)
	}

	//TODO: Test Devices
	//TODO: Test VolumesFrom
	//TODO: Test Volumes

	exists, err = dc.IsContainer(job.Steps[0].Component.Container.Name)
	if err != nil {
		t.Error(err)
	}
	if exists {
		if err = dc.NukeContainerByName(job.Steps[0].Component.Container.Name); err != nil {
			t.Error(err)
		}
	}
}

func TestCreateDownloadContainer(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	image := cfg.GetString("porklock.image")

	tag := cfg.GetString("porklock.tag")

	err = dc.Pull(image, tag)
	if err != nil {
		t.Error(err)
	}

	cName := fmt.Sprintf("/input-0-%s", job.InvocationID)
	exists, err := dc.IsContainer(cName)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(cName); err != nil {
			t.Error(err)
		}
	}

	containerID, err := dc.CreateDownloadContainer(job, &job.Steps[0].Config.Inputs[0], "0")
	if err != nil {
		t.Error(err)
	}

	containerJSON, err := dc.Client.ContainerInspect(dc.ctx, containerID)
	if err != nil {
		t.Error(err)
	}

	if containerJSON.Name != cName {
		t.Errorf("container name was %s instead of %s", containerJSON.Name, cName)
	}

	expected := fmt.Sprintf("%s:%s", image, tag)
	actual := containerJSON.Config.Image
	if actual != expected {
		t.Errorf("Image was %s instead of %s", actual, expected)
	}

	expected = "/de-app-work"
	actual = containerJSON.Config.WorkingDir
	if actual != expected {
		t.Errorf("WorkingDir was %s instead of %s", actual, expected)
	}

	expectedList := job.Steps[0].Config.Inputs[0].Arguments(job.Submitter, job.FileMetadata)
	actualList := []string(containerJSON.Config.Cmd)
	if !reflect.DeepEqual(actualList, expectedList) {
		t.Errorf("Cmd was:\n%#v\ninstead of:\n%#v\n", actualList, expectedList)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}

	expectedBind := fmt.Sprintf("%s:%s:%s", wd, "/de-app-work", "rw")
	if len(containerJSON.HostConfig.Binds) != 1 {
		t.Errorf("Number of binds was %d instead of 1", len(containerJSON.HostConfig.Binds))
	} else {
		actualBind := containerJSON.HostConfig.Binds[0]
		if !reflect.DeepEqual(actualBind, expectedBind) {
			t.Errorf("Bind was:\n%#v\ninstead of:\n%#v", actualBind, expectedBind)
		}
	}

	if _, ok := containerJSON.Config.Labels[model.DockerLabelKey]; !ok {
		t.Error("Label was not set")
	} else {
		actual = containerJSON.Config.Labels[model.DockerLabelKey]
		expected = job.InvocationID
		if actual != expected {
			t.Errorf("The label was set to %s instead of %s", actual, expected)
		}
	}

	expectedLogConfig := container.LogConfig{Type: "none"}
	actualLogConfig := containerJSON.HostConfig.LogConfig
	if actualLogConfig.Type != expectedLogConfig.Type {
		t.Errorf("LogConfig was:\n%#v\ninstead of:\n%#v\n", actualLogConfig, expectedLogConfig)
	}
}

func TestCreateUploadContainer(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	image := cfg.GetString("porklock.image")

	tag := cfg.GetString("porklock.tag")

	containerName := fmt.Sprintf("/output-%s", job.InvocationID)
	exists, err := dc.IsContainer(containerName)
	if err != nil {
		t.Error(err)
	}
	if exists {
		if err = dc.NukeContainerByName(containerName); err != nil {
			t.Error(err)
		}
	}

	containerID, err := dc.CreateUploadContainer(job)
	if err != nil {
		t.Error(err)
	}

	containerJSON, err := dc.Client.ContainerInspect(dc.ctx, containerID)
	if err != nil {
		t.Error(err)
	}

	if containerJSON.Name != containerName {
		t.Errorf("container name was %s instead of %s", containerJSON.Name, containerName)
	}

	expected := fmt.Sprintf("%s:%s", image, tag)
	actual := containerJSON.Config.Image
	if actual != expected {
		t.Errorf("Image was %s instead of %s", actual, expected)
	}

	expected = "/de-app-work"
	actual = containerJSON.Config.WorkingDir
	if actual != expected {
		t.Errorf("WorkingDir was %s instead of %s", actual, expected)
	}

	expectedList := job.FinalOutputArguments()
	actualList := []string(containerJSON.Config.Cmd)
	if !reflect.DeepEqual(actualList, expectedList) {
		t.Errorf("Cmd was:\n%#v\ninstead of:\n%#v\n", actualList, expectedList)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}

	expectedBind := fmt.Sprintf("%s:%s:%s", wd, "/de-app-work", "rw")
	if len(containerJSON.HostConfig.Binds) != 1 {
		t.Errorf("Number of binds was %d instead of 1", len(containerJSON.HostConfig.Binds))
	} else {
		actualBind := containerJSON.HostConfig.Binds[0]
		if !reflect.DeepEqual(actualBind, expectedBind) {
			t.Errorf("Mount was:\n%#v\ninstead of:\n%#v", actualBind, expectedBind)
		}
	}

	if _, ok := containerJSON.Config.Labels[model.DockerLabelKey]; !ok {
		t.Error("Label was not set")
	} else {
		actual = containerJSON.Config.Labels[model.DockerLabelKey]
		expected = job.InvocationID
		if actual != expected {
			t.Errorf("The label was set to %s instead of %s", actual, expected)
		}
	}

	expectedLogConfig := container.LogConfig{Type: "none"}
	actualLogConfig := containerJSON.HostConfig.LogConfig
	if actualLogConfig.Type != expectedLogConfig.Type {
		t.Errorf("LogConfig was:\n%#v\ninstead of:\n%#v\n", actualLogConfig, expectedLogConfig)
	}
}

func TestAttach(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	err = dc.Pull("alpine", "latest")
	if err != nil {
		t.Error(err)
	}

	exists, err := dc.IsContainer(job.Steps[0].Component.Container.Name)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(job.Steps[0].Component.Container.Name); err != nil {
			t.Error(err)
		}
	}

	containerID, err := dc.CreateContainerFromStep(&job.Steps[0], job.InvocationID)
	if err != nil {
		t.Error(err)
	}

	stdout := bytes.NewBufferString("")
	stderr := bytes.NewBufferString("")

	err = dc.Attach(containerID, stdout, stderr)
	if err != nil {
		t.Error(err)
	}

	exists, err = dc.IsContainer(job.Steps[0].Component.Container.Name)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(job.Steps[0].Component.Container.Name); err != nil {
			t.Error(err)
		}
	}
}

func TestRunStep(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	err = dc.Pull("alpine", "latest")
	if err != nil {
		t.Error(err)
	}

	exists, err := dc.IsContainer(job.Steps[0].Component.Container.Name)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(job.Steps[0].Component.Container.Name); err != nil {
			t.Error(err)
		}
	}

	if _, err = os.Stat("logs"); os.IsNotExist(err) {
		err = os.MkdirAll("logs", 0755)
		if err != nil {
			t.Error(err)
		}
	}

	exitCode, err := dc.RunStep(&job.Steps[0], job.InvocationID, 0)
	if err != nil {
		t.Error(err)
	}

	if exitCode != 0 {
		t.Errorf("RunStep's exit code was %d instead of 0\n", exitCode)
	}

	if _, err = os.Stat(job.Steps[0].Stdout("0")); os.IsNotExist(err) {
		t.Error(err)
	}

	if _, err = os.Stat(job.Steps[0].Stderr("0")); os.IsNotExist(err) {
		t.Error(err)
	}

	expected := []byte("This is a test")
	actual, err := ioutil.ReadFile(job.Steps[0].Stdout("0"))
	if err != nil {
		t.Error(err)
	}

	actual = bytes.TrimSpace(actual)
	if !bytes.Equal(actual, expected) {
		t.Errorf("stdout contained '%s' instead of '%s'\n", actual, expected)
	}

	err = os.RemoveAll("logs")
	if err != nil {
		t.Error(err)
	}

	exists, err = dc.IsContainer(job.Steps[0].Component.Container.Name)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(job.Steps[0].Component.Container.Name); err != nil {
			t.Error(err)
		}
	}
}

func TestDownloadInputs(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	image := cfg.GetString("porklock.image")

	tag := cfg.GetString("porklock.tag")

	err = dc.Pull(image, tag)
	if err != nil {
		t.Error(err)
	}

	cName := fmt.Sprintf("input-0-%s", job.InvocationID)
	exists, err := dc.IsContainer(cName)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(cName); err != nil {
			t.Error(err)
		}
	}

	if _, err = os.Stat("logs"); os.IsNotExist(err) {
		err = os.MkdirAll("logs", 0755)
		if err != nil {
			t.Error(err)
		}
	}

	exitCode, err := dc.DownloadInputs(job, &job.Steps[0].Config.Inputs[0], 0)
	if err != nil {
		t.Error(err)
	}

	if exitCode != 0 {
		t.Errorf("DownloadInputs's exit code was %d instead of 0\n", exitCode)
	}

	if _, err = os.Stat(job.Steps[0].Config.Inputs[0].Stdout("0")); os.IsNotExist(err) {
		t.Error(err)
	}

	if _, err = os.Stat(job.Steps[0].Config.Inputs[0].Stderr("0")); os.IsNotExist(err) {
		t.Error(err)
	}

	expected := []byte(strings.Join(
		job.Steps[0].Config.Inputs[0].Arguments(job.Submitter, job.FileMetadata),
		" ",
	))

	actualBytes, err := ioutil.ReadFile(job.Steps[0].Config.Inputs[0].Stdout("0"))
	if err != nil {
		t.Error(err)
	}

	actual := bytes.TrimSpace(actualBytes)
	if !bytes.Equal(actual, expected) {
		t.Errorf("stdout contained '%s' instead of '%s'\n", string(actual), string(expected))
	}

	err = os.RemoveAll("logs")
	if err != nil {
		t.Error(err)
	}

	exists, err = dc.IsContainer(cName)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(cName); err != nil {
			t.Error(err)
		}
	}
}

func TestUploadOutputs(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	image := cfg.GetString("porklock.image")

	tag := cfg.GetString("porklock.tag")

	err = dc.Pull(image, tag)
	if err != nil {
		t.Error(err)
	}

	cName := fmt.Sprintf("output-%s", job.InvocationID)
	exists, err := dc.IsContainer(cName)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(cName); err != nil {
			t.Error(err)
		}
	}

	if _, err = os.Stat("logs"); os.IsNotExist(err) {
		err = os.MkdirAll("logs", 0755)
		if err != nil {
			t.Error(err)
		}
	}

	exitCode, err := dc.UploadOutputs(job)
	if err != nil {
		t.Error(err)
	}

	if exitCode != 0 {
		t.Errorf("UploadOutputs exit code was %d instead of 0\n", exitCode)
	}

	if _, err = os.Stat("logs/logs-stdout-output"); os.IsNotExist(err) {
		t.Error(err)
	}

	if _, err = os.Stat("logs/logs-stderr-output"); os.IsNotExist(err) {
		t.Error(err)
	}

	expected := []byte(strings.Join(
		job.FinalOutputArguments(),
		" ",
	))

	fmt.Printf("%s\n", expected)

	actualBytes, err := ioutil.ReadFile("logs/logs-stdout-output")
	if err != nil {
		t.Error(err)
	}

	actual := bytes.TrimSpace(actualBytes)
	if !bytes.Equal(actual, expected) {
		t.Errorf("stdout contained '%s' instead of '%s'\n", string(actual), string(expected))
	}

	err = os.RemoveAll("logs")
	if err != nil {
		t.Error(err)
	}

	exists, err = dc.IsContainer(cName)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(cName); err != nil {
			t.Error(err)
		}
	}
}

func TestCreateDataContainer(t *testing.T) {
	if !shouldrun() {
		return
	}

	job := inittests(t)

	dc, err := NewDocker(context.Background(), cfg, uri())
	if err != nil {
		t.Error(err)
	}

	image := cfg.GetString("porklock.image")

	tag := cfg.GetString("porklock.tag")

	err = dc.Pull(image, tag)
	if err != nil {
		t.Error(err)
	}

	vf := &model.VolumesFrom{
		Name:          "alpine",
		NamePrefix:    "echo-test",
		Tag:           "latest",
		URL:           "https://hub.docker.com/r/alpine/",
		ReadOnly:      false,
		HostPath:      "/tmp",
		ContainerPath: "/test",
	}

	cName := fmt.Sprintf("%s-%s", vf.NamePrefix, job.InvocationID)
	exists, err := dc.IsContainer(cName)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(cName); err != nil {
			t.Error(err)
		}
	}

	containerID, err := dc.CreateDataContainer(vf, job.InvocationID)
	if err != nil {
		t.Error(err)
	}

	if containerID == "" {
		t.Error("containerID was empty")
	}

	exists, err = dc.IsContainer(cName)
	if err != nil {
		t.Error(err)
	}

	if exists {
		if err = dc.NukeContainerByName(cName); err != nil {
			t.Error(err)
		}
	}
}
