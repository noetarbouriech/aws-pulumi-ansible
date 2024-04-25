package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")

		// A path to the EC2 keypair's private key:
		privateKeyPath := cfg.Require("privateKeyPath")

		// Create a new security group that allows SSH access
		sg, err := ec2.NewSecurityGroup(ctx, "web-secgrp", &ec2.SecurityGroupArgs{
			Description: pulumi.String("Enable SSH access"),
			Ingress: ec2.SecurityGroupIngressArray{
				ec2.SecurityGroupIngressArgs{
					Protocol:    pulumi.String("tcp"),
					FromPort:    pulumi.Int(22),
					ToPort:      pulumi.Int(22),
					CidrBlocks:  pulumi.StringArray{pulumi.String("0.0.0.0/0")},
					Description: pulumi.String("SSH access from anywhere"),
				},
				&ec2.SecurityGroupIngressArgs{
					FromPort: pulumi.Int(80),
					ToPort:   pulumi.Int(80),
					Protocol: pulumi.String("tcp"),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					Description: pulumi.String("HTTP access from anywhere"),
				},
			},
			// Allow all outbound traffic.
			Egress: ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					Protocol: pulumi.String("-1"),
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
				},
			},
		})
		if err != nil {
			return err
		}

		// Create an IAM user with read-only access policy permissions
		user, err := iam.NewUser(ctx, "readOnlyUser", nil)
		if err != nil {
			return err
		}

		// Create an IAM access key for the user
		accessKey, err := iam.NewAccessKey(ctx, "userAccessKey", &iam.AccessKeyArgs{
			User: user.Name,
		})
		if err != nil {
			return err
		}

		_, err = iam.NewUserPolicyAttachment(ctx, "readOnlyUserPolicyAttachment", &iam.UserPolicyAttachmentArgs{
			User:      user.Name,
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/ReadOnlyAccess"),
		})
		if err != nil {
			return err
		}

		// Create a private S3 bucket
		bucket, err := s3.NewBucket(ctx, "myBucket", &s3.BucketArgs{
			Bucket: pulumi.String("webpage-bucket"),
			Acl:    pulumi.String("private"),
		})
		if err != nil {
			return err
		}

		// Upload index.html to the S3 bucket
		_, err = s3.NewBucketObject(ctx, "index.html", &s3.BucketObjectArgs{
			Bucket: bucket.ID(),
			Source: pulumi.NewFileAsset("index.html"),
		})
		if err != nil {
			return err
		}

		// Generate an SSH key for the EC2 instance
		keyPair, err := ec2.NewKeyPair(ctx, "keyPair", &ec2.KeyPairArgs{
			KeyName:   pulumi.String("sshkey"),
			PublicKey: pulumi.String("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDHxJCnp74UyjyW2jwKe1wMQwPrVfL6ywo3jdgPssqYwAiCPLEFlNDPx4A0MitxfjUJc+E//lSz0FfWjFNlP8Qtbjur4Kq0XBUK0I6Vz81Zb3wv9eqh4jesl1kRJQ0isYISqC+poR1jKJV460gJ9RccBU75ZlFNsPOlFDS9jKVTbtQszhP0C8cRS2yMVeDhfsY7Zt3Ub33fYwstw2/5p18PP8ngwHQ6qcPQAUgOb3F61ZA1Yu06fcxTZ/4KwLqdC63keCNf4WgmawPuhMElxNObixTI+Sma8DIH5W7lkDYNUlRG0i6W6n35GSma8SHhCy2VNI0BpF0+TfxofhznlhocToL7yUoRkkXyVdjTt8OJJUzVkz3Ugqm6Xfs6qyeU4sbR3ib0ZSkCW8fvQ1c6hKp/k9Yr0Ci8rt+VyWMBSQTE4RhkKx51DvV+SfZ48tVjba/Vqoe5if1aPLJx8ecFSC4J0fzGE1wnVJG7FfQhukOfJt0hu37yWtdPOlKezWljKa0="),
		})
		if err != nil {
			return err
		}

		// Get the Ubuntu AMI ID
		ami, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
			Filters: []ec2.GetAmiFilter{
				{
					Name:   "name",
					Values: []string{"ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"},
				},
			},
			Owners:     []string{"099720109477"}, // Canonical
			MostRecent: pulumi.BoolRef(true),
		})
		if err != nil {
			return err
		}

		// Create a public EC2 instance (running Ubuntu OS) and assign Elastic/Public IP address
		instance, err := ec2.NewInstance(ctx, "webServer", &ec2.InstanceArgs{
			InstanceType:             pulumi.String("t2.micro"),
			Ami:                      pulumi.String(ami.Id),
			KeyName:                  keyPair.KeyName,
			AssociatePublicIpAddress: pulumi.Bool(true),
			VpcSecurityGroupIds:      pulumi.StringArray{sg.ID()},
		})
		if err != nil {
			return err
		}

		// Allocate and associate an Elastic IP
		// eip, err := ec2.NewEip(ctx, "webEip", nil)
		// if err != nil {
		// 	return err
		// }
		// _, err = ec2.NewEipAssociation(ctx, "eipAssoc", &ec2.EipAssociationArgs{
		// 	AllocationId: eip.AllocationId,
		// 	InstanceId:   instance.ID(),
		// })
		// if err != nil {
		// 	return err
		// }
		// Give our EC2 instance an elastic IP address.
		eip, err := ec2.NewEip(ctx, "webEip", &ec2.EipArgs{
			Instance: instance.ID(),
		})
		if err != nil {
			return err
		}

		// // Render the Ansible playbook using RDS info.
		// renderPlaybookCmd, err := local.NewCommand(ctx, "renderPlaybookCmd", &local.CommandArgs{
		// 	Create: pulumi.String("cat playbook.yaml | envsubst > playbook_rendered.yaml"),
		// 	Environment: pulumi.StringMap{
		// 		"s3_bucket": bucket.Bucket,
		// 	},
		// })
		// if err != nil {
		// 	return err
		// }

		// Run a script to update packages on the remote machine.
		// updatePythonCmd, err := remote.NewCommand(ctx, "updatePythonCmd", &remote.CommandArgs{
		// 	Connection: &remote.ConnectionArgs{
		// 		Host:       eip.PublicIp,
		// 		Port:       pulumi.Float64(22),
		// 		User:       pulumi.String("ubuntu"),
		// 		PrivateKey: privateKey,
		// 	},
		// 	Create: pulumi.String("(sudo apt-get update -y || true);" +
		// 		"(sudo apt-get install python3.5 -y);" +
		// 		"(sudo apt-get install software-properties-common -y)\n"),
		// })
		// if err != nil {
		// 	return err
		// }

		// Finally, play the Ansible playbook to finish installing.
		_, err = local.NewCommand(ctx, "playAnsiblePlaybookCmd", &local.CommandArgs{
			Create: eip.PublicIp.ApplyT(func(publicIp string) (string, error) {
				return fmt.Sprintf("ANSIBLE_HOST_KEY_CHECKING=False ANSIBLE_SSH_RETRIES=6 ansible-playbook "+
					"-u ubuntu "+
					"-i '%v,' "+
					"--private-key %v "+
					"-e s3_bucket=webpage-bucket "+
					"playbook.yaml", publicIp, privateKeyPath), nil
			}).(pulumi.StringOutput),
		}, pulumi.DependsOn([]pulumi.Resource{
			eip,
			instance,
		}))
		if err != nil {
			return err
		}

		// Output the public IP address and SSH key for easy access
		ctx.Export("publicIp", eip.PublicIp)
		ctx.Export("accessKeyId", accessKey.ID())
		ctx.Export("secretAccessKey", accessKey.Secret)

		return nil
	})
}
