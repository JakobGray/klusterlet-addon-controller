// (c) Copyright IBM Corporation 2019, 2020. All Rights Reserved.
// Note to U.S. Government Users Restricted Rights:
// U.S. Government Users Restricted Rights - Use, duplication or disclosure restricted by GSA ADP Schedule
// Contract with IBM Corp.
// Licensed Materials - Property of IBM
//
// Copyright (c) 2020 Red Hat, Inc.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentv1 "github.com/open-cluster-management/endpoint-operator/pkg/apis/agent/v1"
)

// const of appmgr
const (
	ApplicationManager    = "klusterlet-addon-appmgr"
	AppMgr                = "appmgr"
	RequiresHubKubeConfig = true
)

var log = logf.Log.WithName("appmgr")

// IsEnabled - check whether appmgr is enabled
func IsEnabled(instance *agentv1.KlusterletAddonConfig) bool {
	return instance.Spec.ApplicationManagerConfig.Enabled
}

// NewApplicationManagerCR - create CR for component application manager
func NewApplicationManagerCR(
	instance *agentv1.KlusterletAddonConfig,
	namespace string,
) (*agentv1.ApplicationManager, error) {
	labels := map[string]string{
		"app": instance.Name,
	}

	gv := agentv1.GlobalValues{
		ImagePullPolicy: instance.Spec.ImagePullPolicy,
		ImagePullSecret: instance.Spec.ImagePullSecret,
		ImageOverrides:  make(map[string]string, 2),
	}

	imageKey, imageRepository, err := instance.GetImage("subscription")
	if err != nil {
		log.Error(err, "Fail to get Image", "Component.Name", "subscription")
		return nil, err
	}
	gv.ImageOverrides[imageKey] = imageRepository

	return &agentv1.ApplicationManager{
		TypeMeta: metav1.TypeMeta{
			APIVersion: agentv1.SchemeGroupVersion.String(),
			Kind:       "ApplicationManager",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ApplicationManager,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: agentv1.ApplicationManagerSpec{
			FullNameOverride:    ApplicationManager,
			HubKubeconfigSecret: AppMgr + "-hub-kubeconfig",
			ClusterName:         instance.Spec.ClusterName,
			ClusterNamespace:    instance.Spec.ClusterNamespace,
			GlobalValues:        gv,
		},
	}, nil
}
