package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// findSubcommand finds a subcommand by name in a cobra.Command tree.
func findSubcommand(root *cobra.Command, names ...string) *cobra.Command {
	cmd := root
	for _, name := range names {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				cmd = sub
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return cmd
}

// buildTestRoot creates a root command with all admin commands registered.
func buildTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "synapbus"}
	addAdminCommands(root)
	return root
}

func TestWebhookCommandsRegistered(t *testing.T) {
	root := buildTestRoot()

	tests := []struct {
		path []string
	}{
		{[]string{"webhook"}},
		{[]string{"webhook", "register"}},
		{[]string{"webhook", "list"}},
		{[]string{"webhook", "delete"}},
	}

	for _, tt := range tests {
		cmd := findSubcommand(root, tt.path...)
		if cmd == nil {
			t.Errorf("command %v not found", tt.path)
		}
	}
}

func TestK8sCommandsRegistered(t *testing.T) {
	root := buildTestRoot()

	tests := []struct {
		path []string
	}{
		{[]string{"k8s"}},
		{[]string{"k8s", "register"}},
		{[]string{"k8s", "list"}},
		{[]string{"k8s", "delete"}},
	}

	for _, tt := range tests {
		cmd := findSubcommand(root, tt.path...)
		if cmd == nil {
			t.Errorf("command %v not found", tt.path)
		}
	}
}

func TestAttachmentsCommandsRegistered(t *testing.T) {
	root := buildTestRoot()

	tests := []struct {
		path []string
	}{
		{[]string{"attachments"}},
		{[]string{"attachments", "gc"}},
	}

	for _, tt := range tests {
		cmd := findSubcommand(root, tt.path...)
		if cmd == nil {
			t.Errorf("command %v not found", tt.path)
		}
	}
}

func TestWebhookRegisterRequiredFlags(t *testing.T) {
	root := buildTestRoot()
	cmd := findSubcommand(root, "webhook", "register")
	if cmd == nil {
		t.Fatal("webhook register command not found")
	}

	requiredFlags := []string{"url", "events", "secret", "agent"}
	for _, flag := range requiredFlags {
		f := cmd.Flag(flag)
		if f == nil {
			t.Errorf("flag --%s not found on webhook register", flag)
			continue
		}
		ann := f.Annotations
		if ann == nil {
			t.Errorf("flag --%s should be required", flag)
			continue
		}
		if _, ok := ann[cobra.BashCompOneRequiredFlag]; !ok {
			t.Errorf("flag --%s should be required", flag)
		}
	}
}

func TestWebhookDeleteRequiredFlags(t *testing.T) {
	root := buildTestRoot()
	cmd := findSubcommand(root, "webhook", "delete")
	if cmd == nil {
		t.Fatal("webhook delete command not found")
	}

	f := cmd.Flag("id")
	if f == nil {
		t.Fatal("flag --id not found on webhook delete")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("flag --id should be required")
	}
	if _, ok := ann[cobra.BashCompOneRequiredFlag]; !ok {
		t.Fatal("flag --id should be required")
	}
}

func TestK8sRegisterRequiredFlags(t *testing.T) {
	root := buildTestRoot()
	cmd := findSubcommand(root, "k8s", "register")
	if cmd == nil {
		t.Fatal("k8s register command not found")
	}

	requiredFlags := []string{"image", "events", "agent"}
	for _, flag := range requiredFlags {
		f := cmd.Flag(flag)
		if f == nil {
			t.Errorf("flag --%s not found on k8s register", flag)
			continue
		}
		ann := f.Annotations
		if ann == nil {
			t.Errorf("flag --%s should be required", flag)
			continue
		}
		if _, ok := ann[cobra.BashCompOneRequiredFlag]; !ok {
			t.Errorf("flag --%s should be required", flag)
		}
	}
}

func TestK8sDeleteRequiredFlags(t *testing.T) {
	root := buildTestRoot()
	cmd := findSubcommand(root, "k8s", "delete")
	if cmd == nil {
		t.Fatal("k8s delete command not found")
	}

	f := cmd.Flag("id")
	if f == nil {
		t.Fatal("flag --id not found on k8s delete")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("flag --id should be required")
	}
	if _, ok := ann[cobra.BashCompOneRequiredFlag]; !ok {
		t.Fatal("flag --id should be required")
	}
}

func TestK8sRegisterOptionalFlags(t *testing.T) {
	root := buildTestRoot()
	cmd := findSubcommand(root, "k8s", "register")
	if cmd == nil {
		t.Fatal("k8s register command not found")
	}

	optionalFlags := []string{"namespace", "memory", "cpu", "env", "timeout"}
	for _, flag := range optionalFlags {
		f := cmd.Flag(flag)
		if f == nil {
			t.Errorf("optional flag --%s not found on k8s register", flag)
		}
	}
}

func TestExistingCommandsStillPresent(t *testing.T) {
	root := buildTestRoot()

	// Verify existing commands are not broken by our additions.
	existingCmds := []string{"user", "agent", "audit", "backup", "messages", "channels", "conversations", "embeddings", "db", "retention"}
	for _, name := range existingCmds {
		cmd := findSubcommand(root, name)
		if cmd == nil {
			t.Errorf("existing command %q not found after adding new commands", name)
		}
	}
}
