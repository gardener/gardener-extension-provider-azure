// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:generate packr2

package imagevector

import (
	"strings"

	"github.com/gardener/gardener-extension-networking-calico/pkg/calico"

	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gobuffalo/packr/v2"
	"k8s.io/apimachinery/pkg/util/runtime"
)

var imageVector imagevector.ImageVector

func init() {
	box := packr.New("charts", "../../charts")

	imagesYaml, err := box.FindString("images.yaml")
	runtime.Must(err)

	imageVector, err = imagevector.Read(strings.NewReader(imagesYaml))
	runtime.Must(err)

	imageVector, err = imagevector.WithEnvOverride(imageVector)
	runtime.Must(err)
}

// ImageVector is the image vector that contains all the needed images.
func ImageVector() imagevector.ImageVector {
	return imageVector
}

// CalicoCNIImage returns the Calico CNI Image.
func CalicoCNIImage() string {
	image, err := imageVector.FindImage(calico.CNIImageName)
	runtime.Must(err)
	return image.String()
}

// CalicoNodeImage returns the Calico Node image.
func CalicoNodeImage() string {
	image, err := imageVector.FindImage(calico.NodeImageName)
	runtime.Must(err)
	return image.String()
}

// CalicoTyphaImage returns the Calico Typha image.
func CalicoTyphaImage() string {
	image, err := imageVector.FindImage(calico.TyphaImageName)
	runtime.Must(err)
	return image.String()
}

// CalicoKubeControllersImage returns the Calico Kube-controllers image.
func CalicoKubeControllersImage() string {
	image, err := imageVector.FindImage(calico.KubeControllersImageName)
	runtime.Must(err)
	return image.String()
}

// CalicoFlexVolumeDriverImage returns the Calico flexvol image.
func CalicoFlexVolumeDriverImage() string {
	image, err := imageVector.FindImage(calico.PodToDaemonFlexVolumeDriverImageName)
	runtime.Must(err)
	return image.String()
}

// TyphaClusterProportionalAutoscalerImage returns the Calico cluster-proportional-autoscaler image.
func TyphaClusterProportionalAutoscalerImage() string {
	image, err := imageVector.FindImage(calico.TyphaClusterProportionalAutoscalerImageName)
	runtime.Must(err)
	return image.String()
}

// TyphaClusterProportionalVerticalAutoscalerImage returns the Calico cluster-proportional-vertical-autoscaler image.
func TyphaClusterProportionalVerticalAutoscalerImage() string {
	image, err := imageVector.FindImage(calico.TyphaClusterProportionalVerticalAutoscalerImageName)
	runtime.Must(err)
	return image.String()
}
