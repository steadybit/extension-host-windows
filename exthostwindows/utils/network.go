// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package utils

import (
	"context"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"net"
	"slices"
)

func MapToNetworks(ctx context.Context, ipsOrCidrsOrHostnames ...string) ([]net.IPNet, error) {
	includeCidrs, unresolved := akn.ParseCIDRs(ipsOrCidrsOrHostnames)
	resolved, err := Resolve(ctx, unresolved...)
	if err != nil {
		return nil, err
	}
	return append(includeCidrs, akn.IpsToNets(resolved)...), nil
}

func ParsePortRanges(raw []string) ([]akn.PortRange, error) {
	if raw == nil {
		return nil, nil
	}

	var ranges []akn.PortRange

	for _, r := range raw {
		if len(r) == 0 {
			continue
		}
		parsed, err := akn.ParsePortRange(r)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, parsed)
	}

	return ranges, nil
}

// CondenseNetWithPortRange condenses a list of NetWithPortRange
// The way this algorithm works:
// 1. Sort the nwp list ascending by BaseIP and port
// 2. For each nwp in the list create a new nwp with the next neighbor if port-ranges are compatible
// 3. From the new list choose the nwp with the longest prefix length, remove all nwp witch are included in the chosen and add the chosen nwp to the result list
// 4. Repeat 3. until either the list is shorter than limit or no more compatible nwp are found
func CondenseNetWithPortRange(nwps []akn.NetWithPortRange, limit int) []akn.NetWithPortRange {
	if len(nwps) <= limit {
		return nwps
	}

	result := make([]akn.NetWithPortRange, len(nwps))
	copy(result, nwps)
	slices.SortFunc(nwps, akn.NetWithPortRange.Compare)

	var candidates []akn.NetWithPortRange
	for i := 0; i < len(result)-1; i++ {
		if c := getNextMatchingCandidate(result, i); c != nil {
			candidates, _ = insertSorted(candidates, *c, comparePrefixLen)
		}
	}

	slices.SortFunc(candidates, comparePrefixLen)
	for {
		if len(result) <= limit || len(candidates) == 0 {
			return result
		}

		longestPrefix := candidates[0]
		candidates = candidates[1:]

		lenBefore := len(result)
		result = slices.DeleteFunc(result, func(nwp akn.NetWithPortRange) bool {
			return longestPrefix.Contains(nwp)
		})

		//when it was an "old" candidate, and it did not actually remove anything, we can skip it
		if len(result) == lenBefore {
			continue
		}

		var i int
		result, i = insertSorted(result, longestPrefix, akn.NetWithPortRange.Compare)

		//add new candidates resulting from the insterted nwp
		for j := max(i-1, 0); j <= min(i, len(result)-1); j++ {
			if c := getNextMatchingCandidate(result, j); c != nil {
				candidates, _ = insertSorted(candidates, *c, comparePrefixLen)
			}
		}
	}
}

func getNextMatchingCandidate(result []akn.NetWithPortRange, i int) *akn.NetWithPortRange {
	a := result[i]
	for j := i + 1; j < len(result); j++ {
		b := result[j]
		if a.PortRange == b.PortRange {
			if merged := a.Merge(b); !merged.Net.IP.IsUnspecified() {
				return &merged
			}
			return nil
		}
	}
	return nil
}

func insertSorted[S ~[]E, E any](x S, target E, cmp func(E, E) int) (S, int) {
	i, _ := slices.BinarySearchFunc(x, target, cmp)
	return slices.Insert(x, i, target), i
}

func comparePrefixLen(a, b akn.NetWithPortRange) int {
	prefixLenA, _ := a.Net.Mask.Size()
	prefixLenB, _ := b.Net.Mask.Size()
	return prefixLenB - prefixLenA
}
