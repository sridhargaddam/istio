//  Copyright Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package ovnk

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/pkg/test/framework/resource"
	"istio.io/istio/pkg/test/framework/resource/config/cleanup"
	"istio.io/istio/pkg/test/scopes"
)

const (
	policyName        = "udn-namespace-labeler"
	bindingName       = "udn-namespace-labeler"
	cudnNetworkLabel  = "k8s.ovn.org/primary-user-defined-network"
)

// DeployMutatingAdmissionPolicy creates a MutatingAdmissionPolicy and Binding
// that automatically adds UDN labels to namespaces at creation time.
// This eliminates the need to set CUDN labels on every namespace individually.
func DeployMutatingAdmissionPolicy(ctx resource.Context, selectorKey string) error {
	scopes.Framework.Infof("Deploying UDN namespace labeler MutatingAdmissionPolicy (selector=%s)", selectorKey)

	failPolicy := admissionregistrationv1beta1.Fail

	policy := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1beta1.MatchResources{
				ResourceRules: []admissionregistrationv1beta1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1beta1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"namespaces"},
							},
						},
					},
				},
			},
			FailurePolicy:      &failPolicy,
			ReinvocationPolicy: admissionregistrationv1beta1.NeverReinvocationPolicy,
			MatchConditions: []admissionregistrationv1beta1.MatchCondition{
				{
					Name: "only-test-namespaces",
					Expression: `has(object.metadata.labels) && "istio-testing" in object.metadata.labels && object.metadata.labels["istio-testing"] == "istio-test"`,
				},
			},
			Mutations: []admissionregistrationv1beta1.Mutation{
				{
					PatchType: admissionregistrationv1beta1.PatchTypeJSONPatch,
					JSONPatch: &admissionregistrationv1beta1.JSONPatch{
						Expression: fmt.Sprintf(
							`[JSONPatch{op: "add", path: "/metadata/labels/" + jsonpatch.escapeKey("%s"), value: ""},`+
								` JSONPatch{op: "add", path: "/metadata/labels/" + jsonpatch.escapeKey("%s"), value: "true"}]`,
							cudnNetworkLabel, selectorKey,
						),
					},
				},
			},
		},
	}

	binding := &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
			PolicyName: policyName,
		},
	}

	for _, c := range ctx.Clusters() {
		kube := c.Kube()
		client := kube.AdmissionregistrationV1beta1()

		if _, err := client.MutatingAdmissionPolicies().Create(context.TODO(), policy, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("creating MutatingAdmissionPolicy in cluster %s: %w", c.Name(), err)
		}

		if _, err := client.MutatingAdmissionPolicyBindings().Create(context.TODO(), binding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("creating MutatingAdmissionPolicyBinding in cluster %s: %w", c.Name(), err)
		}

		scopes.Framework.Infof("UDN namespace labeler policy deployed in cluster %s", c.Name())
	}

	ctx.CleanupStrategy(cleanup.Conditionally, func() {
		deleteMutatingAdmissionPolicy(ctx)
	})

	return nil
}

func deleteMutatingAdmissionPolicy(ctx resource.Context) {
	for _, c := range ctx.Clusters() {
		client := c.Kube().AdmissionregistrationV1beta1()
		if err := client.MutatingAdmissionPolicyBindings().Delete(context.TODO(), bindingName, metav1.DeleteOptions{}); err != nil {
			scopes.Framework.Warnf("Failed to delete MutatingAdmissionPolicyBinding in cluster %s: %v", c.Name(), err)
		}
		if err := client.MutatingAdmissionPolicies().Delete(context.TODO(), policyName, metav1.DeleteOptions{}); err != nil {
			scopes.Framework.Warnf("Failed to delete MutatingAdmissionPolicy in cluster %s: %v", c.Name(), err)
		}
	}
}
