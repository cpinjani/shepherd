package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/norman/types"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	managementv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/shepherd/pkg/codegen/generator"
	managementSchema "github.com/rancher/shepherd/pkg/schemas/management.cattle.io/v3"
	planv1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/rancher/types/factory"
	controllergen "github.com/rancher/wrangler/v2/pkg/controller-gen"
	"github.com/rancher/wrangler/v2/pkg/controller-gen/args"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func main() {
	err := os.Unsetenv("GOPATH")
	if err != nil {
		return
	}

	controllergen.Run(args.Options{
		OutputPackage: "github.com/rancher/shepherd/pkg/generated",
		Boilerplate:   "pkg/codegen/boilerplate.go.txt",
		Groups: map[string]args.Group{
			appsv1.GroupName: {
				Types: []interface{}{
					appsv1.ControllerRevision{},
					appsv1.Deployment{},
					appsv1.DaemonSet{},
					appsv1.ReplicaSet{},
					appsv1.StatefulSet{},
				},
			},
			corev1.GroupName: {
				Types: []interface{}{
					corev1.Event{},
					corev1.Node{},
					corev1.Namespace{},
					corev1.LimitRange{},
					corev1.ResourceQuota{},
					corev1.Secret{},
					corev1.Service{},
					corev1.ServiceAccount{},
					corev1.Endpoints{},
					corev1.ConfigMap{},
					corev1.PersistentVolume{},
					corev1.PersistentVolumeClaim{},
					corev1.Pod{},
				},
			},
			"management.cattle.io": {
				PackageName: "management.cattle.io",
				Types: []interface{}{
					// All structs with an embedded ObjectMeta field will be picked up
					"./vendor/github.com/rancher/rancher/pkg/apis/management.cattle.io/v3",
					managementv3.ProjectCatalog{},
					managementv3.ClusterCatalog{},
				},
			},
			"catalog.cattle.io": {
				PackageName: "catalog.cattle.io",
				Types: []interface{}{
					catalogv1.App{},
					catalogv1.ClusterRepo{},
					catalogv1.Operation{},
				},
				GenerateClients: true,
			},
			"upgrade.cattle.io": {
				PackageName: "upgrade.cattle.io",
				Types: []interface{}{
					planv1.Plan{},
				},
				GenerateClients: true,
			},
			"provisioning.cattle.io": {
				Types: []interface{}{
					provisioningv1.Cluster{},
				},
				GenerateClients: true,
			},
			"fleet.cattle.io": {
				Types: []interface{}{
					fleet.Bundle{},
					fleet.Cluster{},
					fleet.ClusterGroup{},
				},
			},
			"rke.cattle.io": {
				Types: []interface{}{
					rkev1.RKEBootstrap{},
					rkev1.RKEBootstrapTemplate{},
					rkev1.RKECluster{},
					rkev1.RKEControlPlane{},
					rkev1.ETCDSnapshot{},
					rkev1.CustomMachine{},
				},
				GenerateClients: true,
			},
			"cluster.x-k8s.io": {
				Types: []interface{}{
					capi.Machine{},
					capi.MachineSet{},
					capi.MachineDeployment{},
					capi.Cluster{},
				},
			},
		},
	})

	clusterAPIVersion := &types.APIVersion{Group: capi.GroupVersion.Group, Version: capi.GroupVersion.Version, Path: "/v1"}
	generator.GenerateClient(factory.Schemas(clusterAPIVersion).Init(func(schemas *types.Schemas) *types.Schemas {
		return schemas.MustImportAndCustomize(clusterAPIVersion, capi.Machine{}, func(schema *types.Schema) {
			schema.ID = "cluster.x-k8s.io.machine"
		})
	}), nil)

	generator.GenerateClient(managementSchema.Schemas, map[string]bool{
		"userAttribute": true,
	})

	if err := replaceClientBasePackages(); err != nil {
		panic(err)
	}

	// Comment out this function to avoid replacing the imports in the management controllers
	if err := replaceManagementControllerImports(); err != nil {
		panic(err)
	}
}

// replaceClientBasePackages walks through the zz_generated_client generated by generator.GenerateClient to replace imports from
// "github.com/rancher/norman/clientbase" to "github.com/rancher/shepherd/pkg/clientbase" to use our modified code of the
// session.Session tracking the resources created by the Management Client.
func replaceClientBasePackages() error {
	return filepath.Walk("./clients/rancher/generated", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(info.Name(), "zz_generated_client") {
			input, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			replacement := bytes.Replace(input, []byte("github.com/rancher/norman/clientbase"), []byte("github.com/rancher/shepherd/pkg/clientbase"), -1)

			if err = os.WriteFile(path, replacement, 0666); err != nil {
				return err
			}
		}

		return nil
	})
}

// NOTE: Comment out this function to avoid replacing the imports in the management controllers
func replaceManagementControllerImports() error {
	return filepath.Walk("./pkg/generated/controllers/management.cattle.io", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		var replacement []byte

		if strings.HasSuffix(info.Name(), "go") {
			input, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			replacement = bytes.Replace(input, []byte("github.com/rancher/wrangler/v2/pkg/generic"), []byte("github.com/rancher/shepherd/pkg/wrangler/pkg/generic"), -1)
			if err = os.WriteFile(path, replacement, 0666); err != nil {
				return err
			}

		}

		if strings.HasPrefix(info.Name(), "factory") {
			input, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			replacement = bytes.Replace(input, []byte("c.ControllerFactory())"), []byte("c.ControllerFactory(), c.Opts.TS)"), -1)
			if err = os.WriteFile(path, replacement, 0666); err != nil {
				return err
			}

		}

		if strings.HasPrefix(info.Name(), "interface") {
			input, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			replacement = bytes.Replace(input, []byte("controller.SharedControllerFactory)"), []byte("controller.SharedControllerFactory, ts *session.Session)"), -1)
			if err = os.WriteFile(path, replacement, 0666); err != nil {
				return err
			}
			replacement = bytes.Replace(input, []byte("g.controllerFactory)"), []byte("g.controllerFactory, g.ts)"), -1)
			if err = os.WriteFile(path, replacement, 0666); err != nil {
				return err
			}

			replacement = bytes.Replace(input, []byte("v.controllerFactory)"), []byte("v.controllerFactory, v.ts)"), -1)
			if err = os.WriteFile(path, replacement, 0666); err != nil {
				return err
			}
		}

		return nil
	})
}
