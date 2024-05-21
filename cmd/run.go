/*
Copyright © 2020 Julien B.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"github.com/jbrt/ec2cryptomatic/internal/algorithm"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"

	"github.com/jbrt/ec2cryptomatic/internal/ec2instance"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Encrypt all EBS volumes for the given instances",

	Run: func(cmd *cobra.Command, args []string) {

		instanceID, _ := cmd.Flags().GetString("instance")
		kmsAlias, _ := cmd.Flags().GetString("kmsKeyAlias")
		kmsID, _ := cmd.Flags().GetString("kmsKeyID")
		region, _ := cmd.Flags().GetString("region")
		discard, _ := cmd.Flags().GetBool("discard")
		startInstance, _ := cmd.Flags().GetBool("start")

		fmt.Print("\t\t-=[ EC2Cryptomatic ]=-\n")

		// Load the Shared AWS Configuration (~/.aws/config)
		cfg, err := config.LoadDefaultConfig(
			cmd.Context(),
			config.WithRegion(region),
		)
		if err != nil {
			log.Fatalln("Could not create AWS config: " + err.Error())
		}

		keyID := kmsID
		if keyID == "" {
			keyID = kmsAlias
		}
		kmsService := kms.NewFromConfig(cfg)
		kmsInput := &kms.DescribeKeyInput{KeyId: aws.String(keyID)}

		if _, errorKmsKey := kmsService.DescribeKey(cmd.Context(), kmsInput); errorKmsKey != nil {
			log.Fatalln("Error with this key: " + errorKmsKey.Error())
		}

		ec2Instance, instanceError := ec2instance.New(cfg, instanceID)
		if instanceError != nil {
			log.Fatalln(instanceError)
		}

		if errorAlgorithm := algorithm.EncryptInstance(cmd.Context(), ec2Instance, kmsAlias, discard, startInstance); errorAlgorithm != nil {
			log.Fatalln("/!\\ " + errorAlgorithm.Error())
		}
	},
}

func init() {
	var awsRegion, instanceID, kmsKeyAlias, kmsID string

	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&instanceID, "instance", "i", "", "Instance ID of instance of encrypt (required)")
	runCmd.Flags().StringVarP(&kmsKeyAlias, "kmsKeyAlias", "k", "alias/aws/ebs", "KMS key alias name with format alias/NAME")
	runCmd.Flags().StringVarP(&kmsID, "kmsKeyID", "K", "", "KMS key ID")
	runCmd.Flags().StringVarP(&awsRegion, "region", "r", "", "AWS region (required)")
	runCmd.Flags().BoolP("discard", "d", false, "Discard source volumes after encryption process (default: false)")
	runCmd.Flags().BoolP("start", "s", false, "Start instance after volume encryption (default: false)")
	_ = runCmd.MarkFlagRequired("instance")
	_ = runCmd.MarkFlagRequired("region")
}
