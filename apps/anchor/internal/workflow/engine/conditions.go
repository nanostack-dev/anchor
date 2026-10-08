package engine

import (
	"strings"

	"anchor/internal/domain/workflow"
)

// Holds reports whether every condition is true in the scope. Comparisons
// ignore letter case.
func (s Scope) Holds(conditions []workflow.Condition) (bool, error) {
	for _, condition := range conditions {
		ok, err := s.holds(condition)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func (s Scope) holds(condition workflow.Condition) (bool, error) {
	value, present := s.Lookup(condition.Field)
	actual := strings.ToLower(Stringify(value))
	if present && actual == "" {
		present = false
	}
	if !condition.Operator.NeedsValue() {
		return present == (condition.Operator == workflow.OperatorExists), nil
	}

	expectedRaw, err := s.RenderString(condition.Value)
	if err != nil {
		return false, err
	}
	expected := strings.ToLower(expectedRaw)
	switch condition.Operator {
	case workflow.OperatorEquals:
		return actual == expected, nil
	case workflow.OperatorNotEquals:
		return actual != expected, nil
	case workflow.OperatorContains:
		return present && strings.Contains(actual, expected), nil
	case workflow.OperatorNotContains:
		return !strings.Contains(actual, expected), nil
	case workflow.OperatorStartsWith:
		return present && strings.HasPrefix(actual, expected), nil
	case workflow.OperatorEndsWith:
		return present && strings.HasSuffix(actual, expected), nil
	case workflow.OperatorIn:
		for candidate := range strings.SplitSeq(expected, ",") {
			if present && strings.TrimSpace(candidate) == actual {
				return true, nil
			}
		}
		return false, nil
	case workflow.OperatorExists, workflow.OperatorNotExists:
	}
	return false, unknownOperatorError(condition.Operator)
}
