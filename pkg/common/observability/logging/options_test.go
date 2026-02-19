/*
Copyright 2025 The Kubernetes Authors.

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

package logging

import (
	"flag"
	"testing"

	"github.com/spf13/pflag"
	uberzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewOptions(t *testing.T) {
	opts := NewOptions()
	if opts.LogVerbosity != DEFAULT {
		t.Errorf("Expected LogVerbosity to be %d, got %d", DEFAULT, opts.LogVerbosity)
	}
	if !opts.ZapOptions.Development {
		t.Error("Expected ZapOptions.Development to be true")
	}
}

func TestAddFlags(t *testing.T) {
	opts := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(fs)

	// Check that the -v flag was added
	if fs.Lookup("v") == nil {
		t.Error("Expected -v flag to be added")
	}

	// Check that zap flags were added
	if fs.Lookup(ZapLogLevelFlagName) == nil {
		t.Errorf("Expected %s flag to be added", ZapLogLevelFlagName)
	}
}

func TestComplete(t *testing.T) {
	opts := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(fs)

	// Parse with -v=3
	err := fs.Parse([]string{"-v=3"})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	err = opts.Complete()
	if err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	if opts.LogVerbosity != 3 {
		t.Errorf("Expected LogVerbosity to be 3, got %d", opts.LogVerbosity)
	}

	// ZapOptions.Level should be derived from -v flag
	// When -v=3, the level should be -3 (DebugLevel)
	expectedLevel := zapcore.Level(-3)
	atomicLevel, ok := opts.ZapOptions.Level.(uberzap.AtomicLevel)
	if !ok {
		t.Fatalf("Expected ZapOptions.Level to be zap.AtomicLevel, got %T", opts.ZapOptions.Level)
	}
	actualLevel := atomicLevel.Level()
	if actualLevel != expectedLevel {
		t.Errorf("Expected zap level to be %v, got %v", expectedLevel, actualLevel)
	}
}

func TestComplete_ExplicitZapLevel(t *testing.T) {
	opts := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(fs)

	// Parse with both -v and explicit --zap-log-level
	err := fs.Parse([]string{"-v=5", "--zap-log-level=info"})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	err = opts.Complete()
	if err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	if opts.LogVerbosity != 5 {
		t.Errorf("Expected LogVerbosity to be 5, got %d", opts.LogVerbosity)
	}

	// When --zap-log-level is explicitly set, it should NOT be overridden by -v
	// The explicit flag should take precedence
	zapLogLevelFlag := fs.Lookup(ZapLogLevelFlagName)
	if zapLogLevelFlag == nil {
		t.Fatal("zap-log-level flag not found")
	}
	if !zapLogLevelFlag.Changed {
		t.Error("Expected zap-log-level flag to be marked as changed")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		verbosity   int
		expectError bool
	}{
		{"valid verbosity 0", 0, false},
		{"valid verbosity 2", 2, false},
		{"valid verbosity 5", 5, false},
		{"invalid negative verbosity", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions()
			opts.LogVerbosity = tt.verbosity

			err := opts.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidate_ErrorMessage(t *testing.T) {
	opts := NewOptions()
	opts.LogVerbosity = -1

	err := opts.Validate()
	if err != ErrInvalidLogVerbosity {
		t.Errorf("Expected ErrInvalidLogVerbosity, got %v", err)
	}
}

func init() {
	// Clear any global flags from other tests
	flag.CommandLine = flag.NewFlagSet("", flag.ContinueOnError)
}
