package ec2instance

import (
	"context"
	"github.com/jbrt/ec2cryptomatic/internal/ebsvolume"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var unsupportedInstanceTypes = []string{"c1.", "m1.", "m2.", "t1."}

// Ec2Instance is the main type of that package. Will be returned by new.
// It contains all data relevant for an ec2instance
type Ec2Instance struct {
	InstanceID       *string
	cfg              aws.Config
	ec2client        *ec2.Client
	describeInstance types.Instance
}

// GetEBSMappedVolumes returns EBS volumes mapped with this ec2instance
func (e Ec2Instance) GetEBSMappedVolumes() []types.InstanceBlockDeviceMapping {
	return e.describeInstance.BlockDeviceMappings
}

// GetEBSVolume returns a specific EBS volume with high level methods
func (e Ec2Instance) GetEBSVolume(ctx context.Context, volumeID string) (*ebsvolume.VolumeToEncrypt, error) {
	ebsVolume, volumeError := ebsvolume.New(ctx, e.ec2client, volumeID)
	if volumeError != nil {
		return nil, volumeError
	}
	return ebsVolume, nil
}

// IsStopped will check if the ec2instance is correctly stopped
func (e Ec2Instance) IsStopped() bool {
	if e.describeInstance.State.Name != "stopped" {
		return false
	}
	return true
}

// IsSupportsEncryptedVolumes will check if the ec2instance supports EBS encrypted volumes (not all instances types support that).
func (e Ec2Instance) IsSupportsEncryptedVolumes() bool {
	for _, instance := range unsupportedInstanceTypes {
		if strings.HasPrefix(string(e.describeInstance.InstanceType), instance) {
			return false
		}
	}
	return true
}

// StartInstance will... start the ec2instance. What a surprise ! :-)
func (e Ec2Instance) StartInstance() error {
	log.Println("-- Start ec2instance " + *e.InstanceID)
	input := &ec2.StartInstancesInput{InstanceIds: []string{*e.InstanceID}}
	if _, errStart := e.ec2client.StartInstances(context.TODO(), input); errStart != nil {
		return errStart
	}
	return nil
}

// SwapBlockDevice will swap two EBS volumes from an EC2 ec2instance
func (e Ec2Instance) SwapBlockDevice(ctx context.Context, source types.InstanceBlockDeviceMapping, target types.Volume) error {
	detach := &ec2.DetachVolumeInput{VolumeId: aws.String(*source.Ebs.VolumeId)}
	if _, errDetach := e.ec2client.DetachVolume(ctx, detach); errDetach != nil {
		return errDetach
	}

	w := ec2.NewVolumeAvailableWaiter(e.ec2client)
	err := w.Wait(ctx,
		&ec2.DescribeVolumesInput{VolumeIds: []string{*source.Ebs.VolumeId}},
		time.Minute*5,
	)
	if err != nil {
		return err
	}

	attach := &ec2.AttachVolumeInput{
		Device:     aws.String(*source.DeviceName),
		InstanceId: aws.String(*e.InstanceID),
		VolumeId:   aws.String(*target.VolumeId),
	}

	if _, errAttach := e.ec2client.AttachVolume(ctx, attach); errAttach != nil {
		return errAttach
	}

	//if *source.Ebs.DeleteOnTermination {
	//
	//	mappingSpecification := ec2.InstanceBlockDeviceMappingSpecification{
	//		DeviceName: aws.String(*source.DeviceName),
	//		Ebs: &ec2.EbsInstanceBlockDeviceSpecification{
	//			DeleteOnTermination: aws.Bool(true),
	//			VolumeId:            target.VolumeId,
	//		},
	//	}
	//
	//	attributeInput := ec2.ModifyInstanceAttributeInput{
	//		BlockDeviceMappings: []*ec2.InstanceBlockDeviceMappingSpecification{&mappingSpecification},
	//		InstanceId:          e.InstanceID,
	//	}
	//
	//	requestModify, _ := e.ec2client.ModifyInstanceAttributeRequest(ctx, &attributeInput)
	//
	//	if errorRequest := requestModify.Send(); errorRequest != nil {
	//		return errorRequest
	//	}
	//
	//}

	return nil
}

// New returns a well construct EC2Instance object ec2instance
func New(cfg aws.Config, instanceID string) (*Ec2Instance, error) {
	log.Println("Let's encrypt EC2 instance " + instanceID)

	// Trying to describeInstance the given ec2instance as security mechanism (ec2instance is exists ? credentials are ok ?)
	ec2client := ec2.NewFromConfig(cfg)
	ec2Input := &ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}}

	describe, errDescribe := ec2client.DescribeInstances(context.Background(), ec2Input)
	if errDescribe != nil {
		log.Println("-- Cannot find EC2 instance " + instanceID)
		return nil, errDescribe
	}

	instance := &Ec2Instance{
		InstanceID:       aws.String(instanceID),
		ec2client:        ec2client,
		describeInstance: describe.Reservations[0].Instances[0],
	}

	return instance, nil
}
