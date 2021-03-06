// +build !integration

package proxy

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Setup

type HaProxyTestSuite struct {
	suite.Suite
	TemplatesPath   string
	ConfigsPath     string
	Pid             string
	TemplateContent string
	ServicesContent string
}

// Suite

func TestHaProxyUnitTestSuite(t *testing.T) {
	logPrintf = func(format string, v ...interface{}) {}
	s := new(HaProxyTestSuite)
	s.TemplateContent = `global
    pidfile /var/run/haproxy.pid
    tune.ssl.default-dh-param 2048

defaults
    mode    http
    balance roundrobin

    option  dontlognull
    option  dontlog-normal
    option  http-server-close
    option  forwardfor
    option  redispatch

    errorfile 400 /errorfiles/400.http
    errorfile 403 /errorfiles/403.http
    errorfile 405 /errorfiles/405.http
    errorfile 408 /errorfiles/408.http
    errorfile 429 /errorfiles/429.http
    errorfile 500 /errorfiles/500.http
    errorfile 502 /errorfiles/502.http
    errorfile 503 /errorfiles/503.http
    errorfile 504 /errorfiles/504.http

    maxconn 5000
    timeout connect 5s
    timeout client  20s
    timeout server  20s
    timeout queue   30s
    timeout http-request 5s
    timeout http-keep-alive 15s

    stats enable
    stats refresh 30s
    stats realm Strictly\ Private
    stats auth admin:admin
    stats uri /admin?stats

frontend services
    bind *:80
    bind *:443
    mode http
`
	s.ServicesContent = `

config1 fe content

config2 fe content

config1 be content

config2 be content`
	suite.Run(t, s)
}

func (s *HaProxyTestSuite) SetupTest() {
	s.Pid = "123"
	s.TemplatesPath = "test_configs/tmpl"
	s.ConfigsPath = "test_configs"
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		return nil
	}
	readPidFile = func(fileName string) ([]byte, error) {
		return []byte(s.Pid), nil
	}
}

// AddCertName

func (s HaProxyTestSuite) Test_AddCert_StoresCertificateName() {
	dataOrig := data
	defer func() { data = dataOrig }()
	orig := Instance
	defer func() { Instance = orig }()
	p := HaProxy{}

	p.AddCert("my-cert-3")

	s.Equal(1, len(data.Certs))
	s.Equal(map[string]bool{"my-cert-3": true}, data.Certs)
}

func (s HaProxyTestSuite) Test_AddCert_DoesNotStoreDuplicates() {
	dataOrig := data
	defer func() { data = dataOrig }()
	expected := map[string]bool{"cert-1": true, "cert-2": true, "cert-3": true}
	data.Certs = expected
	orig := Instance
	defer func() { Instance = orig }()
	p := HaProxy{}

	p.AddCert("cert-3")

	s.Equal(3, len(data.Certs))
	s.Equal(expected, data.Certs)
}

// GetCerts

func (s HaProxyTestSuite) Test_GetCerts_ReturnsAllCerts() {
	dataOrig := data
	defer func() { data = dataOrig }()
	p := HaProxy{}
	data.Certs = map[string]bool{}
	expected := map[string]string{}
	for i := 1; i <= 3; i++ {
		certName := fmt.Sprintf("my-cert-%d", i)
		data.Certs[certName] = true
		expected[certName] = fmt.Sprintf("content of the certificate /certs/%s", certName)
	}
	readFileOrig := ReadFile
	defer func() { ReadFile = readFileOrig }()
	ReadFile = func(filename string) ([]byte, error) {
		content := fmt.Sprintf("content of the certificate %s", filename)
		return []byte(content), nil
	}

	actual := p.GetCerts()

	s.EqualValues(expected, actual)
}

// CreateConfigFromTemplates

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ReturnsError_WhenReadDirFails() {
	readConfigsDirOrig := readConfigsDir
	defer func() {
		readConfigsDir = readConfigsDirOrig
	}()
	readConfigsDir = func(dirname string) ([]os.FileInfo, error) {
		return nil, fmt.Errorf("Could not read the directory")
	}

	err := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Error(err)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_WritesCfgContentsIntoFile() {
	var actualData string
	expectedData := fmt.Sprintf(
		"%s%s",
		s.TemplateContent,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsDebug() {
	debugOrig := os.Getenv("DEBUG")
	defer func() { os.Setenv("DEBUG", debugOrig) }()
	os.Setenv("DEBUG", "true")
	var actualData string
	tmpl := strings.Replace(s.TemplateContent, "tune.ssl.default-dh-param 2048", "tune.ssl.default-dh-param 2048\n    debug", -1)
	tmpl = strings.Replace(tmpl, "    option  dontlognull\n    option  dontlog-normal\n", "", -1)
	expectedData := fmt.Sprintf(
		"%s%s",
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsExtraFrontEnd() {
	extraFrontendOrig := os.Getenv("EXTRA_FRONTEND")
	defer func() { os.Setenv("EXTRA_FRONTEND", extraFrontendOrig) }()
	os.Setenv("EXTRA_FRONTEND", "this is an extra content")
	var actualData string
	tmpl := s.TemplateContent + "this is an extra content"
	expectedData := fmt.Sprintf(
		"%s%s",
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEnd() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service-11111 path_beg /path-1 path_beg /path-2 port1111Acl
    acl url_my-service-12222 path_beg /path-3 port2222Acl
    use_backend my-acl-be1111 if url_my-service-11111 my-src-port
    use_backend my-acl-be2222 if url_my-service-12222%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{})
	data.Services["my-service"] = Service{
		ServiceName: "my-service-1",
		PathType:    "path_beg",
		AclName:     "my-acl",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path-1", "/path-2"}, SrcPortAcl: " port1111Acl", SrcPortAclName: " my-src-port"},
			{Port: "2222", ServicePath: []string{"/path-3"}, SrcPortAcl: " port2222Acl"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndTcp() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s

frontend my-service-1_1234
    bind *:1234
    mode tcp
    default_backend my-service-1-be1234%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{})
	data.Services["my-service-1"] = Service{
		ReqMode: "tcp",
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 1234, Port: "4321"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithDomain() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    acl domain_my-service hdr_dom(host) -i domain-1 domain-2
    use_backend my-service-be1111 if url_my-service1111 domain_my-service%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{})
	data.Services["my-service"] = Service{
		ServiceName:   "my-service",
		ServiceDomain: []string{"domain-1", "domain-2"},
		AclName:       "my-service",
		PathType:      "path_beg",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithDomainWildcard() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl domain_my-service hdr_end(host) -i domain-1%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{})
	data.Services["my-service"] = Service{
		ServiceName:   "my-service",
		ServiceDomain: []string{"*domain-1"},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithHttpsPort() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    acl http_my-service src_port 80
    acl https_my-service src_port 443
    use_backend my-service-be1111 if url_my-service1111 http_my-service
    use_backend https-my-service-be1111 if url_my-service1111 https_my-service%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{})
	data.Services["my-service"] = Service{
		ServiceName: "my-service",
		PathType:    "path_beg",
		HttpsPort:   2222,
		AclName:     "my-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsCert() {
	var actualFilename string
	var actualData string
	expectedData := fmt.Sprintf(
		"%s%s",
		strings.Replace(s.TemplateContent, "bind *:443", "bind *:443 ssl crt /certs/my-cert.pem", -1),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{"my-cert.pem": true}).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsBindPorts() {
	bindPortsOrig := os.Getenv("BIND_PORTS")
	defer func() { os.Setenv("BIND_PORTS", bindPortsOrig) }()
	os.Setenv("BIND_PORTS", "1234,4321")
	var actualFilename string
	var actualData string
	expectedData := fmt.Sprintf(
		`%s
    bind *:1234
    bind *:4321%s`,
		s.TemplateContent,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsUserList() {
	var actualData string
	usersOrig := os.Getenv("USERS")
	defer func() { os.Setenv("USERS", usersOrig) }()
	os.Setenv("USERS", "my-user-1:my-password-1,my-user-2:my-password-2")
	expectedData := fmt.Sprintf(
		"%s%s",
		strings.Replace(
			s.TemplateContent,
			"frontend services",
			`userlist defaultUsers
    user my-user-1 insecure-password my-password-1
    user my-user-2 insecure-password my-password-2

frontend services`,
			-1,
		),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ReplacesValuesWithEnvVars() {
	tests := []struct {
		envKey string
		before string
		after  string
		value  string
	}{
		{"TIMEOUT_CONNECT", "timeout connect 5s", "timeout connect 999s", "999"},
		{"TIMEOUT_CLIENT", "timeout client  20s", "timeout client  999s", "999"},
		{"TIMEOUT_SERVER", "timeout server  20s", "timeout server  999s", "999"},
		{"TIMEOUT_QUEUE", "timeout queue   30s", "timeout queue   999s", "999"},
		{"TIMEOUT_HTTP_REQUEST", "timeout http-request 5s", "timeout http-request 999s", "999"},
		{"TIMEOUT_HTTP_KEEP_ALIVE", "timeout http-keep-alive 15s", "timeout http-keep-alive 999s", "999"},
		{"STATS_USER", "stats auth admin:admin", "stats auth my-user:admin", "my-user"},
		{"STATS_PASS", "stats auth admin:admin", "stats auth admin:my-pass", "my-pass"},
	}
	for _, t := range tests {
		timeoutOrig := os.Getenv(t.envKey)
		os.Setenv(t.envKey, t.value)
		var actualFilename string
		var actualData string
		expectedData := fmt.Sprintf(
			"%s%s",
			strings.Replace(s.TemplateContent, t.before, t.after, -1),
			s.ServicesContent,
		)
		writeFile = func(filename string, data []byte, perm os.FileMode) error {
			actualFilename = filename
			actualData = string(data)
			return nil
		}

		NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

		s.Equal(expectedData, actualData)

		os.Setenv(t.envKey, timeoutOrig)
	}
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_WritesMockDataIfConfigsAreNotPresent() {
	var actualData string
	readConfigsDirOrig := readConfigsDir
	defer func() {
		readConfigsDir = readConfigsDirOrig
	}()
	readConfigsDir = func(dirname string) ([]os.FileInfo, error) {
		return []os.FileInfo{}, nil
	}
	expectedData := fmt.Sprintf(
		"%s%s",
		s.TemplateContent,
		`

    acl url_dummy path_beg /dummy
    use_backend dummy-be if url_dummy

backend dummy-be
    server dummy 1.1.1.1:1111 check`,
	)

	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ReturnsError_WhenReadConfigsFileFails() {
	readConfigsFileOrig := readConfigsFile
	defer func() {
		readConfigsFile = readConfigsFileOrig
	}()
	readConfigsFile = func(dirname string) ([]byte, error) {
		return nil, fmt.Errorf("Could not read the directory")
	}

	err := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).CreateConfigFromTemplates()

	s.Error(err)
}

// ReadConfig

func (s *HaProxyTestSuite) Test_ReadConfig_ReturnsConfig() {
	var actualFilename string
	expectedFilename := fmt.Sprintf("%s/haproxy.cfg", s.ConfigsPath)
	expectedData := `template content

config1 content

config2 content`
	readFileOrig := ReadFile
	defer func() { ReadFile = readFileOrig }()
	ReadFile = func(filename string) ([]byte, error) {
		actualFilename = filename
		return []byte(expectedData), nil
	}

	actualData, _ := NewHaProxy(s.TemplatesPath, s.ConfigsPath, map[string]bool{}).ReadConfig()

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s *HaProxyTestSuite) Test_ReadConfig_ReturnsError_WhenReadFileFails() {
	readFileOrig := ReadFile
	defer func() { ReadFile = readFileOrig }()
	ReadFile = func(filename string) ([]byte, error) {
		return []byte{}, fmt.Errorf("This is an error")
	}

	_, actual := HaProxy{}.ReadConfig()

	s.Error(actual)
}

// Reload

func (s *HaProxyTestSuite) Test_Reload_ReadsPidFile() {
	var actual string
	readPidFile = func(fileName string) ([]byte, error) {
		actual = fileName
		return []byte("12345"), nil
	}

	HaProxy{}.Reload()

	s.Equal("/var/run/haproxy.pid", actual)
}

func (s *HaProxyTestSuite) Test_Reload_ReturnsError_WhenHaCommandFails() {
	cmdRunHa = func(cmd *exec.Cmd) error {
		return fmt.Errorf("This is an error")
	}

	err := HaProxy{}.Reload()

	s.Error(err)
}

func (s *HaProxyTestSuite) Test_Reload_ReturnsError_WhenReadPidFails() {
	readPidFile = func(fileName string) ([]byte, error) {
		return []byte(""), fmt.Errorf("This is an error")
	}

	err := HaProxy{}.Reload()

	s.Error(err)
}

func (s *HaProxyTestSuite) Test_Reload_RunsRunCmd() {
	actual := HaProxyTestSuite{}.mockHaExecCmd()
	expected := []string{
		"haproxy",
		"-f",
		"/cfg/haproxy.cfg",
		"-D",
		"-p",
		"/var/run/haproxy.pid",
		"-sf",
		s.Pid,
	}

	HaProxy{}.Reload()

	s.Equal(expected, *actual)
}

// AddService

func (s *HaProxyTestSuite) Test_AddService_AddsService() {
	s1 := Service{ServiceName: "my-service-1"}
	s2 := Service{ServiceName: "my-service-2"}
	s3 := Service{ServiceName: "my-service-2", ServiceDomain: []string{"domain-1", "domain-2"}}
	p := NewHaProxy("anything", "doesn't", map[string]bool{}).(HaProxy)

	p.AddService(s1)
	p.AddService(s2)
	p.AddService(s3)

	s.Len(data.Services, 2)
	s.Equal(data.Services[s1.ServiceName], s1)
	s.Equal(data.Services[s3.ServiceName], s3)
}

// RemoveService

func (s *HaProxyTestSuite) Test_AddService_RemovesService() {
	s1 := Service{ServiceName: "my-service-1"}
	s2 := Service{ServiceName: "my-service-2"}
	s3 := Service{ServiceName: "my-service-2", ServiceDomain: []string{"domain-1", "domain-2"}}
	p := NewHaProxy("anything", "doesn't", map[string]bool{}).(HaProxy)

	p.AddService(s1)
	p.AddService(s2)
	p.AddService(s3)
	p.RemoveService("my-service-1")

	s.Len(data.Services, 1)
	s.Equal(data.Services[s3.ServiceName], s3)
}

// Mocks

func (s HaProxyTestSuite) mockHaExecCmd() *[]string {
	var actualCommand []string
	cmdRunHa = func(cmd *exec.Cmd) error {
		actualCommand = cmd.Args
		return nil
	}
	return &actualCommand
}
