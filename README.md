# Test using Pulumi to deploy on EC2

Deploy nginx on EC2 using Pulumi and Ansible.

## Requirements
- Pulumi
- SSH key
- Ansible
- AWS acces key env variables

## Install
```sh
pulumi config set aws-pulumi-ansible:privateKeyPath ~/.ssh/aws_rsa
pulumi config set aws:region eu-west-3
pulumi up
```
