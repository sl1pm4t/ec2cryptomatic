package ebsvolume

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// VolumeToEncrypt contains all needed information for encrypting an EBS volume
type VolumeToEncrypt struct {
	volumeID *string
	client   *ec2.Client
	describe types.Volume
}

// getTagSpecifications will returns tags from volumes by filtering out AWS specific tags (aws:xxx)
func (v VolumeToEncrypt) getTagSpecifications() []types.TagSpecification {
	var tags []types.Tag

	if v.describe.Tags == nil {
		return nil
	}

	for _, val := range v.describe.Tags {
		if !strings.HasPrefix(*val.Key, "aws:") {
			tags = append(tags, val)
		}
	}

	return []types.TagSpecification{{ResourceType: types.ResourceTypeVolume, Tags: tags}}

}

// takeSnapshot will take a snapshot from the given EBS volume & wait until this snapshot is completed
func (v VolumeToEncrypt) takeSnapshot(ctx context.Context) (*types.Snapshot, error) {
	snapShotInput := &ec2.CreateSnapshotInput{
		Description: aws.String("EC2Cryptomatic temporary snapshot for " + *v.volumeID),
		VolumeId:    v.describe.VolumeId,
	}

	createSnapOut, errSnapshot := v.client.CreateSnapshot(
		ctx,
		snapShotInput,
	)
	if errSnapshot != nil {
		return nil, errSnapshot
	}

	descrSnapInput := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{*createSnapOut.SnapshotId},
	}

	w := ec2.NewSnapshotCompletedWaiter(v.client)
	err := w.Wait(ctx, descrSnapInput, time.Minute*5)
	if err != nil {
		return nil, err
	}

	snapshot, err := v.client.DescribeSnapshots(ctx, descrSnapInput)
	if err != nil {
		return nil, err
	}

	return &snapshot.Snapshots[0], nil
}

// DeleteVolume will delete the given EBS volume
func (v VolumeToEncrypt) DeleteVolume() error {
	log.Println("---> Delete volume " + *v.volumeID)
	if _, errDelete := v.client.DeleteVolume(context.TODO(), &ec2.DeleteVolumeInput{VolumeId: v.volumeID}); errDelete != nil {
		return errDelete
	}
	return nil
}

// EncryptVolume will produce an encrypted version of the EBS volume
func (v VolumeToEncrypt) EncryptVolume(ctx context.Context, kmsKeyID string) (*types.Volume, error) {
	log.Println("---> Start encryption process for volume " + *v.volumeID)
	encrypted := true
	snapshot, errSnapshot := v.takeSnapshot(ctx)
	if errSnapshot != nil {
		return nil, errSnapshot
	}

	volumeInput := &ec2.CreateVolumeInput{
		AvailabilityZone: aws.String(*v.describe.AvailabilityZone),
		SnapshotId:       aws.String(*snapshot.SnapshotId),
		VolumeType:       v.describe.VolumeType,
		Encrypted:        &encrypted,
		KmsKeyId:         aws.String(kmsKeyID),
	}

	// Adding tags if needed
	if tagsWithoutAwsDedicatedTags := v.getTagSpecifications(); tagsWithoutAwsDedicatedTags != nil {
		volumeInput.TagSpecifications = tagsWithoutAwsDedicatedTags
	}

	// If EBS volume is IO, let's get the IOPs parameter
	if strings.HasPrefix(string(v.describe.VolumeType), "io") {
		log.Println("---> This volumes is IO one let's set IOPs to ", *v.describe.Iops)
		volumeInput.Iops = v.describe.Iops
	}

	createVolOut, errVolume := v.client.CreateVolume(context.TODO(), volumeInput)
	if errVolume != nil {
		return nil, errVolume
	}

	descrVolInput := &ec2.DescribeVolumesInput{
		VolumeIds: []string{*createVolOut.VolumeId},
	}

	w := ec2.NewVolumeAvailableWaiter(v.client)
	err := w.Wait(ctx, descrVolInput, time.Minute*5)
	if err != nil {
		return nil, err
	}

	volume, err := v.client.DescribeVolumes(context.Background(), descrVolInput)
	if err != nil {
		return nil, err
	}

	// Before ends, delete the temporary snapshot
	_, _ = v.client.DeleteSnapshot(ctx, &ec2.DeleteSnapshotInput{SnapshotId: snapshot.SnapshotId})

	return &volume.Volumes[0], nil
}

// IsEncrypted will returns true if the given EBS volume is already encrypted
func (v VolumeToEncrypt) IsEncrypted() bool {
	return *v.describe.Encrypted
}

// New returns a well construct EC2Instance object ec2instance
func New(ctx context.Context, ec2Client *ec2.Client, volumeID string) (*VolumeToEncrypt, error) {
	// Trying to describe the given ec2instance as security mechanism (ec2instance is exists ? credentials are ok ?)
	volumeInput := &ec2.DescribeVolumesInput{VolumeIds: []string{volumeID}}
	describe, errDescribe := ec2Client.DescribeVolumes(ctx, volumeInput)
	if errDescribe != nil {
		log.Println("---> Cannot get information from volume " + volumeID)
		return nil, errDescribe
	}

	volume := &VolumeToEncrypt{
		volumeID: aws.String(volumeID),
		client:   ec2Client,
		describe: describe.Volumes[0],
	}

	return volume, nil
}
