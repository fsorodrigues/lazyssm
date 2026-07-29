package process

import (
	"reflect"
	"testing"
)

func TestBuildAuthCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		path    string
		args    []string
		wantErr bool
	}{
		{
			name:    "empty",
			command: "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			command: "   \t  ",
			wantErr: true,
		},
		{
			name:    "simple command",
			command: "aws-mfa",
			path:    "aws-mfa",
		},
		{
			name:    "simple command with args",
			command: "aws-mfa --profile dev",
			path:    "aws-mfa",
			args:    []string{"--profile", "dev"},
		},
		{
			name:    "equals arg",
			command: "aws-mfa --profile=dev",
			path:    "aws-mfa",
			args:    []string{"--profile=dev"},
		},
		{
			name:    "double quoted arg",
			command: "foo --arg \"some value\"",
			path:    "foo",
			args:    []string{"--arg", "some value"},
		},
		{
			name:    "single quoted arg",
			command: "foo --arg 'some value'",
			path:    "foo",
			args:    []string{"--arg", "some value"},
		},
		{
			name:    "escaped quote",
			command: `foo --arg "some \"quoted\" value"`,
			path:    "foo",
			args:    []string{"--arg", `some "quoted" value`},
		},
		{
			name:    "unterminated quote",
			command: `foo "unterminated`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := BuildAuthCommand(tt.command)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Args[0] != tt.path {
				t.Fatalf("argv[0] = %q, want %q", cmd.Args[0], tt.path)
			}
			gotArgs := cmd.Args[1:]
			wantArgs := tt.args
			if wantArgs == nil {
				wantArgs = []string{}
			}
			if !reflect.DeepEqual(gotArgs, wantArgs) {
				t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
			}
		})
	}
}

func TestParseAuthCommand(t *testing.T) {
	args, err := ParseAuthCommand(`foo --arg "some value"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"foo", "--arg", "some value"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}
