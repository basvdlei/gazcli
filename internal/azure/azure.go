// Package azure contains the Azure API calls
package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/google/uuid"
)

const defaultTimeout = 30 * time.Second

type subscriptionID string

// Session holds the active session login credentials and related settings.
type Session struct {
	ctx         context.Context
	timeout     time.Duration
	principalID string
	credential  *azidentity.DefaultAzureCredential
}

// NewSession returns default credentials using the information from the OS
// environment.
// TODO: extract the users principal id somehow. Current solution is to extract
// it with: `az ad signed-in-user show | jq .id`
func NewSession(ctx context.Context, principalID string) (*Session, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return &Session{
		ctx:         ctx,
		timeout:     defaultTimeout,
		principalID: principalID,
		credential:  cred,
	}, nil
}

// Subscription returns all enabled subscriptions.
func (s *Session) Subscriptions() (map[string]subscriptionID, error) {
	sc, err := armsubscription.NewSubscriptionsClient(s.credential, nil)
	if err != nil {
		return nil, err
	}
	pager := sc.NewListPager(&armsubscription.SubscriptionsClientListOptions{})
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	var subs = make(map[string]subscriptionID)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return subs, err
		}
		for _, v := range nextResult.Value {
			if v == nil || v.DisplayName == nil || v.SubscriptionID == nil || v.State == nil {
				return subs, errors.New("unexptected nil value returned")
			}
			if *v.State != armsubscription.SubscriptionStateEnabled {
				// ignore all disabled, deleted, etc subscriptions.
				continue
			}
			if _, found := subs[*v.DisplayName]; found {
				log.Printf("WARNING: duplicate subscription ignored: %s",
					*v.DisplayName)
			} else {
				subs[*v.DisplayName] = subscriptionID(*v.SubscriptionID)
			}
		}
	}
	return subs, nil
}

// RolesForSubscription lists the available roles that have
// RoleEligibilitySchedules for the given subscription name.
func (s *Session) RolesForSubscription(subscriptionName string) ([]string, error) {
	subs, err := s.Subscriptions()
	if err != nil {
		return []string{}, err
	}
	sub, found := subs[subscriptionName]
	if !found {
		return []string{}, errors.New("subscription not found")
	}

	res, err := s.roleEligibilitySchedules(fmt.Sprintf("/subscriptions/%s", sub))
	if err != nil {
		return []string{}, err
	}
	roles := []string{}
	for k := range res {
		roles = append(roles, k)
	}
	return roles, nil
}

// RoleEligibilitySchedules returns all the RoleEligibitlySchedules available
// for the given scope.
// XXX example scope: "subscriptions/"+subscriptionID,
func (s *Session) roleEligibilitySchedules(scope string) (map[string]*armauthorization.RoleEligibilitySchedule, error) {
	rsi, err := armauthorization.NewRoleEligibilitySchedulesClient(s.credential, nil)
	if err != nil {
		return nil, err
	}
	pager := rsi.NewListForScopePager(
		scope,
		&armauthorization.RoleEligibilitySchedulesClientListForScopeOptions{
			//TODO add a filter since we are getting more roles
			//then the portal and some duplicates.
		},
	)
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	var r = make(map[string]*armauthorization.RoleEligibilitySchedule)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return r, err
		}
		for _, v := range nextResult.Value {
			if v == nil || v.Properties == nil ||
				v.Properties.ExpandedProperties == nil ||
				v.Properties.ExpandedProperties.RoleDefinition == nil ||
				v.Properties.ExpandedProperties.RoleDefinition.DisplayName == nil {
				return r, errors.New("unexptected nil value returned")
			}
			if _, found := r[*v.Properties.ExpandedProperties.RoleDefinition.DisplayName]; found {
				log.Printf("WARNING: duplicate eligiblity ignored for role %s",
					*v.Properties.ExpandedProperties.RoleDefinition.DisplayName)
			} else {
				r[*v.Properties.ExpandedProperties.RoleDefinition.DisplayName] = v
			}
		}
	}
	return r, nil
}

// ActiveRoleAssignment will schedule a RoleAssignment for the given
// roleDisplayName scoped to the subscription.
func (s *Session) ActiveRoleAssignment(subscriptionName, roleDisplayName, justifiction string, duration time.Duration) error {
	subs, err := s.Subscriptions()
	if err != nil {
		return err
	}
	subscriptionID, found := subs[subscriptionName]
	if !found {
		return errors.New("subscription not found")
	}

	res, err := s.roleEligibilitySchedules(fmt.Sprintf("subscriptions/%s", subscriptionID))
	if err != nil {
		return err
	}
	re, found := res[roleDisplayName]
	if !found {
		return errors.New("Role Eligibility Schedule not found")
	}
	/*
		if b, err := json.MarshalIndent(re, "", "\t"); err == nil {
			log.Printf("%s\n", b)
		}
	*/

	// Make sure no nil values returned for the fields used below.
	if re == nil || re.ID == nil || re.Properties == nil ||
		re.Properties.ExpandedProperties == nil ||
		re.Properties.ExpandedProperties.RoleDefinition == nil ||
		re.Properties.RoleDefinitionID == nil {
		return errors.New("unexptected nil value returned")
	}

	requestType := armauthorization.RequestTypeSelfActivate
	startTime := time.Now()
	expirationType := armauthorization.TypeAfterDuration
	expirationDuration := fmt.Sprintf("PT%dM", int(duration.Minutes()))
	req := armauthorization.RoleAssignmentScheduleRequest{
		Properties: &armauthorization.RoleAssignmentScheduleRequestProperties{
			PrincipalID:      &s.principalID,
			RequestType:      &requestType,
			RoleDefinitionID: re.Properties.RoleDefinitionID,
			Justification:    &justifiction,
			ScheduleInfo: &armauthorization.RoleAssignmentScheduleRequestPropertiesScheduleInfo{
				StartDateTime: &startTime,
				Expiration: &armauthorization.RoleAssignmentScheduleRequestPropertiesScheduleInfoExpiration{
					Type:     &expirationType,
					Duration: &expirationDuration,
				},
			},
			LinkedRoleEligibilityScheduleID: re.ID,
		},
	}

	scope := fmt.Sprintf("/subscriptions/%s", subscriptionID)
	guid, err := uuid.NewUUID()
	if err != nil {
		return err
	}

	rasc, err := armauthorization.NewRoleAssignmentScheduleRequestsClient(
		s.credential, nil)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	resp, err := rasc.Create(
		ctx,
		scope,
		guid.String(),
		req,
		&armauthorization.RoleAssignmentScheduleRequestsClientCreateOptions{},
	)
	if err != nil {
		return err
	}
	if b, err := json.MarshalIndent(resp, "", "\t"); err == nil {
		log.Printf("%s\n", b)
	}
	return nil
}
