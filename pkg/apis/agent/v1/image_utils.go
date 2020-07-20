// (c) Copyright IBM Corporation 2019, 2020. All Rights Reserved.
// Note to U.S. Government Users Restricted Rights:
// U.S. Government Users Restricted Rights - Use, duplication or disclosure restricted by GSA ADP Schedule
// Contract with IBM Corp.
// Licensed Materials - Property of IBM
//
// Copyright (c) 2020 Red Hat, Inc.

package v1

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/open-cluster-management/endpoint-operator/version"
)

var defaultComponentImageKeyMap = map[string]string{
	"cert-policy-controller":          "cert_policy_controller",
	"addon-operator":                  "endpoint_component_operator",
	"coredns":                         "coredns",
	"deployable":                      "multicluster_operators_deployable",
	"iam-policy-controller":           "iam_policy_controller",
	"policy-controller":               "config_policy_controller",
	"governance-policy-spec-sync":     "governance_policy_spec_sync",
	"governance-policy-status-sync":   "governance_policy_status_sync",
	"governance-policy-template-sync": "governance_policy_template_sync",
	"router":                          "management_ingress",
	"search-collector":                "search_collector",
	"service-registry":                "multicloud_manager",
	"subscription":                    "multicluster_operators_subscription",
	"work-manager":                    "multicloud_manager",
}

//Manifest contains the manifest.
//The Manifest is loaded using the LoadManifest method.
var Manifest manifest

var versionList []*semver.Version

var log = logf.Log.WithName("image_utils")

type manifest struct {
	Images []manifestElement `json:"inline"`
}

type manifestElement struct {
	ImageKey        string `json:"image-key,omitempty"`
	ImageName       string `json:"image-name,omitempty"`
	ImageVersion    string `json:"image-version,omitempty"`
	ImageTag        string `json:"image-tag,omitempty"`
	ImageDigest     string `json:"image-digest,omitempty"`
	ImageRepository string `json:"image-remote,omitempty"`
	GitSha256       string `json:"git-sha256,omitempty"`
	GitRepository   string `json:"git-repository,omitempty"`
}

func init() {
	Manifest.Images = make([]manifestElement, 0)

	manifestPath := filepath.Join("image-manifests", version.Version+".json")
	homeDir := os.Getenv("IMAGE_MANIFEST_PATH")

	if homeDir != "" {
		manifestPath = filepath.Join(homeDir, manifestPath)
	}

	err := LoadManifest(manifestPath)
	if err != nil {
		log.Error(err, "Error while reading the manifest")
	}

	err = GetVersionsManifest("image-manifests")
	if err != nil {
		log.Error(err, "Error while getting version lists")
	}
}

// GetImage returns the image.Image,  for the specified component return error if information not found
func (instance KlusterletAddonConfig) GetImage(component string) (imageKey, imageRepository string, err error) {

	if v, ok := defaultComponentImageKeyMap[component]; ok {
		imageKey = v
	} else {
		return "", "", fmt.Errorf("unable to locate default image name for component %s", component)
	}

	imageManifest, err := getImageManifest(imageKey)
	if err != nil {
		return "", "", err
	}

	imageKey = imageManifest.ImageKey

	if instance.Spec.ImageRegistry != "" {
		imageRepository = instance.Spec.ImageRegistry
	} else {
		imageRepository = imageManifest.ImageRepository
	}

	imageRepository = imageRepository + "/" + imageManifest.ImageName + "@" + imageManifest.ImageDigest

	return imageKey, imageRepository, nil
}

//getImageManifest returns the *manifestElement and nil if not found
//Return an error only if the manifest is malformed
func getImageManifest(imageKey string) (*manifestElement, error) {
	for i, im := range Manifest.Images {
		if im.ImageKey == imageKey {
			return &Manifest.Images[i], nil
		}
	}
	return nil, fmt.Errorf("ImageManifest not found for %s", imageKey)
}

//LoadManifest returns the *manifestElement and nil if not found
//Return an error only if the manifest is malformed
func LoadManifest(manifestPath string) error {
	//Check if already loaded
	if len(Manifest.Images) != 0 {
		return nil
	}

	b, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(b, &Manifest.Images)
	if err != nil {
		return err
	}

	return nil
}

// GetVersionsManifest returns the available version list of klusterlet
func GetVersionsManifest(manifestPath string) error {
	files, err := ioutil.ReadDir(manifestPath)
	if err != nil {
		log.Error(err, "Fail to read manifest directory", "path", manifestPath)
		return err
	}

	c, err := semver.NewConstraint(">= 2.0.0")
	if err != nil {
		log.Error(err, "Invalid semantic constraint")
	}

	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), ".json") {
			version := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			v, err := semver.NewVersion(version)
			if err != nil {
				log.Error(err, "Invalid semantic version found in image-manifests")
				return err
			}
			if c.Check(v) {
				versionList = append(versionList, v)
			}
		}
	}

	return nil
}

// GetAvailableVersions returns the available version list of klusterlet
func (instance KlusterletAddonConfig) GetAvailableVersions() ([]*semver.Version, error) {
	if len(versionList) == 0 {
		return nil, fmt.Errorf("Version list is empty")
	}

	return versionList, nil
}
