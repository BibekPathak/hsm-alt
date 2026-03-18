package policy

import (
	"context"
	"fmt"
)

type Rule struct {
	Name      string
	Condition Condition
	Action    Action
}

type Condition struct {
	Type     string
	Field    string
	Operator string
	Value    interface{}
}

type Action struct {
	Type  string
	Value interface{}
}

type PolicyEngine struct {
	rules []Rule
}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		rules: make([]Rule, 0),
	}
}

func (e *PolicyEngine) AddRule(rule Rule) {
	e.rules = append(e.rules, rule)
}

func (e *PolicyEngine) Evaluate(ctx context.Context, request *PolicyRequest) (*PolicyDecision, error) {
	for _, rule := range e.rules {
		if e.matchesCondition(rule.Condition, request) {
			return &PolicyDecision{
				Allowed: rule.Action.Type == "allow",
				Reason:  rule.Name,
			}, nil
		}
	}

	return &PolicyDecision{
		Allowed: true,
		Reason:  "default",
	}, nil
}

func (e *PolicyEngine) matchesCondition(cond Condition, request *PolicyRequest) bool {
	switch cond.Field {
	case "amount":
		if amount, ok := request.Data["amount"].(float64); ok {
			if threshold, ok := cond.Value.(float64); ok {
				switch cond.Operator {
				case "gt":
					return amount > threshold
				case "lt":
					return amount < threshold
				case "eq":
					return amount == threshold
				}
			}
		}
	case "signers":
		if signers, ok := request.Data["signers"].(int); ok {
			if threshold, ok := cond.Value.(int); ok {
				return signers >= threshold
			}
		}
	}
	return false
}

type PolicyRequest struct {
	ClusterID string
	Data      map[string]interface{}
}

type PolicyDecision struct {
	Allowed bool
	Reason  string
}

type RateLimitConfig struct {
	MaxRequestsPerMinute int64
	MaxValuePerHour      int64
}

func ValidateRateLimit(cfg RateLimitConfig, request *PolicyRequest) error {
	return fmt.Errorf("rate limit not implemented")
}
