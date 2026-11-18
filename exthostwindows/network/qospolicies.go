// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/rs/zerolog/log"
)

const (
	qosPolicyPrefix = "STEADYBIT_QOS_"
	// qosPolicyClass is the WMI class behind the NetQos cmdlets (New-NetQosPolicy, Remove-NetQosPolicy, ...).
	qosPolicyClass      = "MSFT_NetQosPolicySettingData"
	qosPolicyPrecedence = 255
)

var (
	// The names are embedded in WQL queries, so only characters without a meaning in WQL are allowed.
	qosPolicyNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	// '_' is a wildcard in WQL LIKE patterns, '[_]' matches the character itself.
	steadybitQosPolicies = "Name LIKE '" + strings.ReplaceAll(qosPolicyPrefix, "_", "[_]") + "%'"
)

type QosPolicy struct {
	Name          string
	BitsPerSecond uint64
	DstPrefix     string
	// DstPortFrom and DstPortTo are zero when the policy applies to all ports.
	DstPortFrom uint16
	DstPortTo   uint16
}

func applyQosPolicies(ctx context.Context, policies []QosPolicy, mode Mode) error {
	logCurrentQosPolicies(ctx, "before")
	defer logCurrentQosPolicies(ctx, "after")

	if mode == ModeAdd {
		return createQosPolicies(ctx, policies)
	}
	names := make([]string, 0, len(policies))
	for _, p := range policies {
		names = append(names, p.Name)
	}
	return removeQosPolicies(ctx, names)
}

func logCurrentQosPolicies(ctx context.Context, when string) {
	if !log.Trace().Enabled() {
		return
	}
	policies, err := listSteadybitQosPolicies(ctx)
	if err != nil {
		log.Trace().Err(err).Msg("failed to get current QoS policies")
	} else {
		log.Trace().Str("when", when).Interface("policies", policies).Msg("current QoS policies")
	}
}

func createQosPolicies(ctx context.Context, policies []QosPolicy) error {
	for _, policy := range policies {
		if !qosPolicyNamePattern.MatchString(policy.Name) {
			return fmt.Errorf("invalid QoS policy name %q", policy.Name)
		}
		// Attack state prepared by an earlier version carries no rate.
		if policy.BitsPerSecond < 8 {
			return fmt.Errorf("invalid rate of QoS policy %s: %d bit/s, prepare the attack again", policy.Name, policy.BitsPerSecond)
		}
	}

	return withQosWmi(ctx, func(svc *ole.IDispatch) error {
		class, err := oleutil.CallMethod(svc, "Get", qosPolicyClass)
		if err != nil {
			return err
		}
		defer func() { _ = class.Clear() }()

		for _, policy := range policies {
			if err := createQosPolicy(class.ToIDispatch(), policy); err != nil {
				return fmt.Errorf("failed to create QoS policy %s: %w", policy.Name, err)
			}
		}
		return nil
	})
}

func createQosPolicy(class *ole.IDispatch, policy QosPolicy) error {
	instance, err := oleutil.CallMethod(class, "SpawnInstance_")
	if err != nil {
		return err
	}
	defer func() { _ = instance.Clear() }()

	properties := map[string]any{
		"Name":       policy.Name,
		"Precedence": int32(qosPolicyPrecedence),
		// WMI scripting passes 64-bit integers as strings.
		"ThrottleRateAction":        strconv.FormatUint(policy.BitsPerSecond, 10),
		"IPDstPrefixMatchCondition": policy.DstPrefix,
	}
	if policy.DstPortFrom != 0 {
		properties["IPDstPortStartMatchCondition"] = int32(policy.DstPortFrom)
		properties["IPDstPortEndMatchCondition"] = int32(policy.DstPortTo)
	}
	for name, value := range properties {
		result, err := oleutil.PutProperty(instance.ToIDispatch(), name, value)
		if err != nil {
			return fmt.Errorf("failed to set %s: %w", name, err)
		}
		_ = result.Clear()
	}

	result, err := oleutil.CallMethod(instance.ToIDispatch(), "Put_")
	if err != nil {
		return err
	}
	return result.Clear()
}

// removeQosPolicies removes the policies with the given names. A policy that does not exist counts as removed, so a
// repeated stop of an attack succeeds.
func removeQosPolicies(ctx context.Context, names []string) error {
	if len(names) == 0 {
		return nil
	}
	conditions := make([]string, 0, len(names))
	for _, name := range names {
		if !qosPolicyNamePattern.MatchString(name) {
			return fmt.Errorf("invalid QoS policy name %q", name)
		}
		conditions = append(conditions, "Name = '"+name+"'")
	}
	if _, err := deleteQosPolicies(ctx, strings.Join(conditions, " OR ")); err != nil {
		return fmt.Errorf("failed to remove QoS policies %s: %w", strings.Join(names, ", "), err)
	}
	return nil
}

func removeSteadybitQosPolicies(ctx context.Context) error {
	removed, err := deleteQosPolicies(ctx, steadybitQosPolicies)
	if len(removed) > 0 {
		log.Error().Strs("policies", removed).Msg("Found leftover QoS policies, removed them")
	}
	if err != nil {
		return fmt.Errorf("failed to remove leftover QoS policies: %w", err)
	}
	return nil
}

// deleteQosPolicies deletes the policies matching the WQL condition in one query and returns the names it deleted.
// A failed delete does not stop the others.
func deleteQosPolicies(ctx context.Context, condition string) ([]string, error) {
	var removed []string
	err := withQosWmi(ctx, func(svc *ole.IDispatch) error {
		var deleteErrs error
		queryErr := forEachQosPolicy(svc, "SELECT * FROM "+qosPolicyClass+" WHERE "+condition, func(item *ole.IDispatch) error {
			name := propertyString(item, "Name")
			result, err := oleutil.CallMethod(item, "Delete_")
			if err != nil {
				deleteErrs = errors.Join(deleteErrs, fmt.Errorf("failed to delete %s: %w", name, err))
				return nil
			}
			removed = append(removed, name)
			return result.Clear()
		})
		return errors.Join(queryErr, deleteErrs)
	})
	if errors.Is(err, errWmiTimeout) {
		return nil, err
	}
	return removed, err
}

func listSteadybitQosPolicies(ctx context.Context) ([]QosPolicy, error) {
	var policies []QosPolicy
	query := "SELECT Name, ThrottleRateAction, IPDstPrefixMatchCondition, IPDstPortStartMatchCondition, IPDstPortEndMatchCondition FROM " +
		qosPolicyClass + " WHERE " + steadybitQosPolicies
	err := withQosWmi(ctx, func(svc *ole.IDispatch) error {
		return forEachQosPolicy(svc, query, func(item *ole.IDispatch) error {
			policy := QosPolicy{
				Name:      propertyString(item, "Name"),
				DstPrefix: propertyString(item, "IPDstPrefixMatchCondition"),
			}
			policy.BitsPerSecond, _ = strconv.ParseUint(propertyString(item, "ThrottleRateAction"), 10, 64)
			from, _ := strconv.ParseUint(propertyString(item, "IPDstPortStartMatchCondition"), 10, 16)
			to, _ := strconv.ParseUint(propertyString(item, "IPDstPortEndMatchCondition"), 10, 16)
			policy.DstPortFrom, policy.DstPortTo = uint16(from), uint16(to)
			policies = append(policies, policy)
			return nil
		})
	})
	if err != nil {
		// errWmiTimeout: the abandoned call may still append to policies, so it is not read.
		return nil, fmt.Errorf("failed to list QoS policies: %w", err)
	}
	return policies, nil
}

func forEachQosPolicy(svc *ole.IDispatch, query string, fn func(item *ole.IDispatch) error) error {
	result, err := oleutil.CallMethod(svc, "ExecQuery", query)
	if err != nil {
		return err
	}
	defer func() { _ = result.Clear() }()

	return oleutil.ForEach(result.ToIDispatch(), func(v *ole.VARIANT) error {
		defer func() { _ = v.Clear() }()
		return fn(v.ToIDispatch())
	})
}

// propertyString returns the property as string, or "" when it is not set.
func propertyString(item *ole.IDispatch, name string) string {
	v, err := oleutil.GetProperty(item, name)
	if err != nil {
		return ""
	}
	defer func() { _ = v.Clear() }()
	if v.Value() == nil {
		return ""
	}
	return fmt.Sprint(v.Value())
}
