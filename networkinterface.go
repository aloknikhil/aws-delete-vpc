package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"
)

func allocationIds(addresses []types.Address) []string {
	allocationIds := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address.AllocationId != nil {
			allocationIds = append(allocationIds, *address.AllocationId)
		}
	}
	return allocationIds
}

func deleteNetworkInterfaces(ctx context.Context, client *ec2.Client, networkInterfaces []types.NetworkInterface) (errs error) {
	for _, networkInterface := range networkInterfaces {
		if networkInterface.NetworkInterfaceId == nil {
			continue
		}

		// Release any Elastic IPs associated with the network interface.
		output, err := client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("network-interface-id"),
					Values: []string{*networkInterface.NetworkInterfaceId},
				},
			},
		})
		errs = multierr.Append(errs, err)
		if err == nil {
			log.Err(err).
				Strs("AllocationIds", allocationIds(output.Addresses)).
				Msg("DescribeAddresses")
			for _, address := range output.Addresses {
				_, err := client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
					AllocationId: address.AllocationId,
				})
				log.Err(err).
					Str("AllocationId", *address.AllocationId).
					Str("PublicIp", *address.PublicIp).
					Msg("ReleaseAddress")
				errs = multierr.Append(errs, err)
			}
		} else {
			log.Err(err).
				Msg("DescribeAddresses")
		}

		// Detach the NetworkInterface.
		if networkInterface.Attachment != nil && networkInterface.Attachment.AttachmentId != nil {
			_, err := client.DetachNetworkInterface(ctx, &ec2.DetachNetworkInterfaceInput{
				AttachmentId: networkInterface.Attachment.AttachmentId,
			})
			log.Err(err).
				Str("AttachmentId", *networkInterface.Attachment.AttachmentId).
				Msg("DetachNetworkInterface")
			errs = multierr.Append(errs, err)
			if err != nil {
				continue
			}

			// FIXME Wait for detachment somehow
		}

		// Delete the NetworkInterface.
		_, err = client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: networkInterface.NetworkInterfaceId,
		})
		log.Err(err).
			Str("NetworkInterfaceId", *networkInterface.NetworkInterfaceId).
			Msg("DeleteNetworkInterface")
		errs = multierr.Append(errs, err)
	}
	return
}

func listNetworkInterfaces(ctx context.Context, client *ec2.Client, vpcId string) ([]types.NetworkInterface, error) {
	input := ec2.DescribeNetworkInterfacesInput{
		Filters: ec2VpcFilter(vpcId),
	}
	var networkInterfaces []types.NetworkInterface
	for {
		output, err := client.DescribeNetworkInterfaces(ctx, &input)
		if err != nil {
			return nil, err
		}
		networkInterfaces = append(networkInterfaces, output.NetworkInterfaces...)
		if output.NextToken == nil {
			return networkInterfaces, nil
		}
		input.NextToken = output.NextToken
	}
}

func networkInterfaceIds(networkInterfaces []types.NetworkInterface) []string {
	networkInterfaceIds := make([]string, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		if networkInterface.NetworkInterfaceId != nil {
			networkInterfaceIds = append(networkInterfaceIds, *networkInterface.NetworkInterfaceId)
		}
	}
	return networkInterfaceIds
}
