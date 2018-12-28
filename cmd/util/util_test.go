/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"bytes"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/version"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func startTestHTTPServer(returnError bool, response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if returnError {
			http.Error(w, response, 400)
		} else {
			fmt.Fprintf(w, response)
		}
	}))
}

func TestFormatError(t *testing.T) {
	var testErr error
	if _, err := FormatError(testErr); err == nil {
		t.Fatalf("FormatError should have errored with a nil error input")
	}
	testErr = fmt.Errorf("Not a valid error to format as there is no stacktrace")

	if out, err := FormatError(testErr); err == nil {
		t.Fatalf("FormatError should have errored with a non pkg/errors error (no stacktrace info): %s", out)
	}

	testErr = errors.New("TestFormatError 1")
	errors.Wrap(testErr, "TestFormatError 2")
	errors.Wrap(testErr, "TestFormatError 3")

	_, err := FormatError(testErr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestMarshallError(t *testing.T) {
	testErr := errors.New("TestMarshallError 1")
	errors.Wrap(testErr, "TestMarshallError 2")
	errors.Wrap(testErr, "TestMarshallError 3")

	errMsg, _ := FormatError(testErr)
	if _, err := MarshallError(errMsg, "default", version.GetVersion()); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestUploadError(t *testing.T) {
	testErr := errors.New("TestUploadError 1")
	errors.Wrap(testErr, "TestUploadError 2")
	errors.Wrap(testErr, "TestUploadError 3")
	errMsg, _ := FormatError(testErr)
	jsonErrMsg, _ := MarshallError(errMsg, "default", version.GetVersion())

	server := startTestHTTPServer(false, "http test")
	if err := UploadError(jsonErrMsg, server.URL); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	server = startTestHTTPServer(true, "failed to write report")
	if err := UploadError(jsonErrMsg, server.URL); err == nil {
		t.Fatalf("UploadError should have errored from a 400 response")
	}
}

func TestReportError(t *testing.T) {
	testErr := errors.New("TestError 1")

	server := startTestHTTPServer(false, "http test")
	if err := ReportError(testErr, server.URL); err != nil {
		t.Fatalf("ReportError")
	}
}

func revertLookPath(l LookPath) {
	lookPath = l
}

func fakeLookPathFound(string) (string, error) { return "/usr/local/bin/kubectl", nil }
func fakeLookPathError(string) (string, error) { return "", errors.New("") }

func TestKubectlDownloadMsg(t *testing.T) {
	var tests = []struct {
		description    string
		lp             LookPath
		goos           string
		matches        string
		noOutput       bool
		warningEnabled bool
	}{
		{
			description:    "No output when binary is found windows",
			goos:           "windows",
			lp:             fakeLookPathFound,
			noOutput:       true,
			warningEnabled: true,
		},
		{
			description:    "No output when binary is found darwin",
			goos:           "darwin",
			lp:             fakeLookPathFound,
			noOutput:       true,
			warningEnabled: true,
		},
		{
			description:    "windows kubectl not found, has .exe in output",
			goos:           "windows",
			lp:             fakeLookPathError,
			matches:        ".exe",
			warningEnabled: true,
		},
		{
			description:    "linux kubectl not found",
			goos:           "linux",
			lp:             fakeLookPathError,
			matches:        "WantKubectlDownloadMsg",
			warningEnabled: true,
		},
		{
			description:    "warning disabled",
			goos:           "linux",
			lp:             fakeLookPathError,
			noOutput:       true,
			warningEnabled: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.description, func(t *testing.T) {
			defer revertLookPath(lookPath)

			// Remember the original config value and revert to it.
			origConfig := viper.GetBool(config.WantKubectlDownloadMsg)
			defer func() {
				viper.Set(config.WantKubectlDownloadMsg, origConfig)
			}()
			viper.Set(config.WantKubectlDownloadMsg, test.warningEnabled)
			lookPath = test.lp
			var b bytes.Buffer
			MaybePrintKubectlDownloadMsg(test.goos, &b)
			actual := b.String()
			if actual != "" && test.noOutput {
				t.Errorf("Got output, but kubectl binary was found")
			}
			if !strings.Contains(actual, test.matches) {
				t.Errorf("Output did not contain substring expected got output %s", actual)
			}
		})
	}
}

func TestGetKubeConfigPath(t *testing.T) {
	var tests = []struct {
		input string
		want  string
	}{
		{
			input: "/home/fake/.kube/.kubeconfig",
			want:  "/home/fake/.kube/.kubeconfig",
		},
		{
			input: "/home/fake/.kube/.kubeconfig:/home/fake2/.kubeconfig",
			want:  "/home/fake/.kube/.kubeconfig",
		},
	}

	for _, test := range tests {
		os.Setenv(clientcmd.RecommendedConfigPathEnvVar, test.input)
		if result := GetKubeConfigPath(); result != test.want {
			t.Errorf("Expected first splitted chunk, got: %s", result)
		}
	}
}
