// Proof of Concepts of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// This is a Cloud Driver

package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"

	idrv "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces"
	irs "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces/resources"
)

type GCPTagHandler struct {
	Region     idrv.RegionInfo
	Ctx        context.Context
	Credential idrv.CredentialInfo

	ComputeClient   *compute.Service
	ContainerClient *container.Service
}

var (
	supportRSType = map[irs.RSType]interface{}{
		irs.VM: nil, irs.DISK: nil, irs.CLUSTER: nil,
	}
)

func validateSupportRS(resType irs.RSType) error {
	if _, ok := supportRSType[resType]; !ok {
		return errors.New("unsupported resources type")
	}
	return nil
}

func (t *GCPTagHandler) getVm(resIID irs.IID) (*compute.Instance, error) {
	vm, err := t.ComputeClient.Instances.Get(t.Credential.ProjectID, t.Region.Zone, resIID.SystemId).Do()
	if err != nil {
		return nil, err
	}

	return vm, nil
}

func (t *GCPTagHandler) getDisk(resIID irs.IID) (*compute.Disk, error) {
	disk, err := GetDiskInfo(t.ComputeClient, t.Credential, t.Region, resIID.SystemId)
	if err != nil {
		return nil, err
	}

	return disk, nil
}

func (t *GCPTagHandler) getCluster(resIID irs.IID) (*container.Cluster, error) {
	parent := getParentClusterAtContainer(t.Credential.ProjectID, t.Region.Zone, resIID.SystemId)
	cluster, err := t.ContainerClient.Projects.Locations.Clusters.Get(parent).Do()
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

func (t *GCPTagHandler) AddTag(resType irs.RSType, resIID irs.IID, tag KeyValue) (KeyValue, error) {
	err := validateSupportRS(resType)
	errRes := KeyValue{}
	if err != nil {
		return errRes, err
	}

	projectId := t.Credential.ProjectID
	zone := t.Region.Zone
	switch resType {
	case irs.VM:
		vm, err := t.getVm(resIID)
		if err != nil {
			return errRes, err
		}

		existLabels := vm.Labels
		existLabels[tag.Key] = tag.Value

		req := &compute.InstancesSetLabelsRequest{
			LabelFingerprint: vm.Fingerprint,
			Labels:           existLabels,
		}

		op, err := t.ComputeClient.Instances.SetLabels(projectId, zone, resIID.SystemId, req).Do()

		if err != nil {
			return errRes, err
		}

		if op.Error != nil {
			return errRes, fmt.Errorf("operation failed: %v", op.Error.Errors)
		}

		return tag, nil
	case irs.DISK:

		disk, err := t.getDisk(resIID)
		if err != nil {
			return errRes, err
		}

		existLabels := disk.Labels
		existLabels[tag.Key] = tag.Value

		req := &compute.ZoneSetLabelsRequest{
			LabelFingerprint: disk.LabelFingerprint,
			Labels:           existLabels,
		}

		op, err := t.ComputeClient.Disks.SetLabels(projectId, zone, resIID.SystemId, req).Do()

		if err != nil {
			return errRes, err
		}

		if op.Error != nil {
			return errRes, fmt.Errorf("operation failed: %v", op.Error.Errors)
		}

		return tag, nil
	case irs.CLUSTER:
		cluster, err := t.getCluster(resIID)
		if err != nil {
			return errRes, err
		}

		existLabels := cluster.ResourceLabels
		existLabels[tag.Key] = tag.Value

		name := getParentClusterAtContainer(projectId, zone, resIID.SystemId)
		req := &container.SetLabelsRequest{
			ClusterId:        resIID.SystemId,
			LabelFingerprint: cluster.LabelFingerprint,
			Name:             name,
			ProjectId:        projectId,
			Zone:             zone,
			ResourceLabels:   existLabels,
		}
		op, err := t.ContainerClient.Projects.Locations.Clusters.SetResourceLabels(name, req).Do()

		if err != nil {
			return errRes, err
		}

		if op.Error != nil {
			return errRes, fmt.Errorf("operation failed: %v", op.Error.Message)
		}

		return tag, nil
	default:
		return tag, errors.New("unsupported resource type")
	}
}

func (t *GCPTagHandler) waitForOperation(o *compute.Operation) error {
	cnt := 10
	projectID := t.Credential.ProjectID
	zone := t.Region.Zone
	for cnt < 0 {
		if strings.ToUpper(o.Status) == "DONE" {
			if o.Error != nil {
				return fmt.Errorf("operation failed: %v", o.Error.Errors)
			}
			return nil
		}

		time.Sleep(2 * time.Second)
		op, err := t.ComputeClient.ZoneOperations.Get(projectID, zone, o.Name).Do()
		if err != nil {
			return fmt.Errorf("failed to get operation status: %v", err)
		}
		cnt--
		o = op
	}

	return errors.New("operation has not been finished.")
}

func (t *GCPTagHandler) ListTag(resType irs.RSType, resIID irs.IID) ([]KeyValue, error) {
	err := validateSupportRS(resType)
	res := []KeyValue{}
	if err != nil {
		return res, err
	}

	projectID := t.Credential.ProjectID
	zone := t.Region.Zone
	switch resType {
	case irs.VM:
		vm, err := t.ComputeClient.Instances.Get(projectID, zone, resIID.SystemId).Do()
		if err != nil {
			return res, err
		}
		for k, v := range vm.Labels {
			kv := KeyValue{
				Key:   k,
				Value: v,
			}
			res = append(res, kv)
		}
		return res, nil
	case irs.DISK:
		disk, err := GetDiskInfo(t.ComputeClient, t.Credential, t.Region, resIID.SystemId)
		if err != nil {
			return res, err
		}

		for k, v := range disk.Labels {
			kv := KeyValue{
				Key:   k,
				Value: v,
			}
			res = append(res, kv)
		}
		return res, nil
	case irs.CLUSTER:
		parent := getParentClusterAtContainer(projectID, zone, resIID.SystemId)
		cluster, err := t.ContainerClient.Projects.Locations.Clusters.Get(parent).Do()
		if err != nil {
			return res, err
		}

		for k, v := range cluster.ResourceLabels {
			kv := KeyValue{
				Key:   k,
				Value: v,
			}
			res = append(res, kv)
		}
		return res, nil
	default:
		return res, errors.New("unsupport resources type")
	}
}
func (t *GCPTagHandler) GetTag(resType irs.RSType, resIID irs.IID, key string) (KeyValue, error) {
	labels, err := t.ListTag(resType, resIID)
	res := KeyValue{}
	if err != nil {
		return res, err
	}

	for _, l := range labels {
		if l.Key == key {
			res.Key = l.Key
			res.Value = l.Value
			return res, nil
		}
	}

	return res, nil
}
func (t *GCPTagHandler) RemoveTag(resType irs.RSType, resIID irs.IID, key string) (bool, error) {
	err := validateSupportRS(resType)
	if err != nil {
		return false, err
	}

	projectId := t.Credential.ProjectID
	zone := t.Region.Zone
	switch resType {
	case irs.VM:
		vm, err := t.getVm(resIID)
		if err != nil {
			return false, err
		}

		existLabels := vm.Labels
		if _, ok := existLabels[key]; ok {
			delete(existLabels, key)
		}

		req := &compute.InstancesSetLabelsRequest{
			LabelFingerprint: vm.Fingerprint,
			Labels:           existLabels,
		}

		op, err := t.ComputeClient.Instances.SetLabels(projectId, zone, resIID.SystemId, req).Do()

		if err != nil {
			return false, err
		}

		if op.Error != nil {
			return false, fmt.Errorf("operation failed: %v", op.Error.Errors)
		}

		return true, nil
	case irs.DISK:

		disk, err := t.getDisk(resIID)
		if err != nil {
			return false, err
		}

		existLabels := disk.Labels
		if _, ok := existLabels[key]; ok {
			delete(existLabels, key)
		}
		req := &compute.ZoneSetLabelsRequest{
			LabelFingerprint: disk.LabelFingerprint,
			Labels:           existLabels,
		}

		op, err := t.ComputeClient.Disks.SetLabels(projectId, zone, resIID.SystemId, req).Do()

		if err != nil {
			return false, err
		}

		if op.Error != nil {
			return false, fmt.Errorf("operation failed: %v", op.Error.Errors)
		}

		return true, nil
	case irs.CLUSTER:
		cluster, err := t.getCluster(resIID)
		if err != nil {
			return false, err
		}

		existLabels := cluster.ResourceLabels
		if _, ok := existLabels[key]; ok {
			delete(existLabels, key)
		}

		name := getParentClusterAtContainer(projectId, zone, resIID.SystemId)
		req := &container.SetLabelsRequest{
			ClusterId:        resIID.SystemId,
			LabelFingerprint: cluster.LabelFingerprint,
			Name:             name,
			ProjectId:        projectId,
			Zone:             zone,
			ResourceLabels:   existLabels,
		}
		op, err := t.ContainerClient.Projects.Locations.Clusters.SetResourceLabels(name, req).Do()

		if err != nil {
			return false, err
		}

		if op.Error != nil {
			return false, fmt.Errorf("operation failed: %v", op.Error.Message)
		}

		return true, nil
	default:
		return false, errors.New("unsupported resource type")
	}
}
func (t *GCPTagHandler) FindTag(resType irs.RSType, keyword string) ([]*irs.TagInfo, error) {
	// err := validateSupportRS(resType)
	// errRes := []*irs.TagInfo{}
	// if err != nil {
	// 	return errRes, err
	// }

	// projectId := t.Credential.ProjectID
	// zone := t.Region.Zone
	// switch resType {
	// case irs.VM:
	// 	vms, err := t.ComputeClient.Instances.List(projectId, zone).Do()
	// 	if err != nil {
	// 		return errRes, err
	// 	}

	// 	for _, i := range vms.Items {
	// 		irs.TagInfo{
	// 			ResType: resType,
	// 			ResIId: irs.IID{
	// 				NameId: "",
	// 				SystemId: "",
	// 			},
	// 		}
	// 		for k, v := range i.Labels {
	// 			if strings.Contains(k, keyword) || strings.Contains(v, keyword) {

	// 					irs.KeyValue{

	// 					}

	// 			}
	// 		}
	// 	}

	// case irs.DISK:
	// 	disks, err := t.ComputeClient.Disks.List(projectId, zone).Do()
	// 	if err != nil {
	// 		return errRes, err
	// 	}

	// case irs.CLUSTER:
	// 	parent := getParentAtContainer(projectId, zone)
	// 	clusters, err := t.ContainerClient.Projects.Locations.Clusters.List(parent).Do()
	// 	if err != nil {
	// 		return errRes, err
	// 	}

	// default:

	// }

	return []*irs.TagInfo{}, nil
}
