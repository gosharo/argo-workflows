package cluster

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow"
)

func newGetRemoteResourcesCommand() *cobra.Command {
	var (
		localNamespace  string
		remoteNamespace string
		read            bool
		write           bool
	)
	cmd := &cobra.Command{
		Use:   "get-remote-resources local_cluster remote_cluster",
		Short: "print the resource manifests to set-up the the remote cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}
			localCluster, remoteCluster := args[0], args[1]

			name := remoteServiceAccountName(localNamespace, localCluster, remoteNamespace, read, write)

			const (
				rbacAPIGroup   = "rbac.authorization.k8s.io"
				rbacAPIVersion = rbacAPIGroup + "/v1"
				readRole       = "argo-read"
				writeRole      = "argo-write"
			)

			resources := []metav1.Object{
				&apiv1.ServiceAccount{
					TypeMeta:   metav1.TypeMeta{Kind: "ServiceAccount", APIVersion: "v1"},
					ObjectMeta: metav1.ObjectMeta{Name: name},
				},
				&rbacv1.Role{
					TypeMeta:   metav1.TypeMeta{Kind: "Role", APIVersion: rbacAPIVersion},
					ObjectMeta: metav1.ObjectMeta{Name: readRole},
					Rules: []rbacv1.PolicyRule{
						{Verbs: []string{"list", "watch"}, APIGroups: []string{workflow.Group}, Resources: []string{workflow.WorkflowTaskResultPlural}},
						{Verbs: []string{"list", "watch"}, APIGroups: []string{""}, Resources: []string{"pods", "pods/exec"}},
					},
				},
				&rbacv1.Role{
					TypeMeta:   metav1.TypeMeta{Kind: "Role", APIVersion: rbacAPIVersion},
					ObjectMeta: metav1.ObjectMeta{Name: writeRole},
					Rules: []rbacv1.PolicyRule{
						{Verbs: []string{"deletecollection"}, APIGroups: []string{workflow.Group}, Resources: []string{workflow.WorkflowTaskResultPlural}},
						{Verbs: []string{"create", "patch", "delete"}, APIGroups: []string{""}, Resources: []string{"pods", "pods/exec"}},
					},
				},
			}

			subjects := []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: name,
				},
			}
			if read {
				resources = append(resources,
					&rbacv1.RoleBinding{
						TypeMeta:   metav1.TypeMeta{Kind: "RoleBinding", APIVersion: rbacAPIVersion},
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("argo-read.%s", name)},
						Subjects:   subjects,
						RoleRef:    rbacv1.RoleRef{APIGroup: rbacAPIGroup, Kind: "Role", Name: readRole},
					},
				)
			}
			if write {
				resources = append(resources,
					&rbacv1.RoleBinding{
						TypeMeta:   metav1.TypeMeta{Kind: "RoleBinding", APIVersion: rbacAPIVersion},
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("argo-write.%s", name)},
						Subjects:   subjects,
						RoleRef:    rbacv1.RoleRef{APIGroup: rbacAPIGroup, Kind: "Role", Name: writeRole},
					},
				)
			}

			_, _ = os.Stdout.WriteString("# This is an auto-generated file. DO NOT EDIT\n")
			_, _ = os.Stdout.WriteString(fmt.Sprintf("# namespace %q in cluster %q to namespace %q in cluster %q\n", localNamespace, localCluster, remoteNamespace, remoteCluster))

			for _, resource := range resources {
				_, _ = os.Stdout.WriteString("---\n")
				data, _ := yaml.Marshal(resource)
				_, _ = os.Stdout.Write(data)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&localNamespace, "local-namespace", "", "restrict to this local namespace (empty for all namespaces)")
	cmd.Flags().StringVar(&remoteNamespace, "remote-namespace", "", "restrict the this remote namespace (empty for all namespaces)")
	cmd.Flags().BoolVar(&read, "read", false, "create roles with read permissions")
	cmd.Flags().BoolVar(&write, "write", false, "create roles with write permission")
	return cmd
}

func remoteServiceAccountName(localNamespace, localCluster, remoteNamespace string, read, write bool) string {
	suffix := ""
	if read && write {
		suffix = "rw"
	} else if read {
		suffix = "ro"
	} else if write {
		suffix = "wo"
	}
	return fmt.Sprintf("argo.%s.%s.%s.%s", localNamespace, localCluster, remoteNamespace, suffix)
}