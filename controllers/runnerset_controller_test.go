package controllers

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"math/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	actionsv1alpha1 "github.com/summerwind/actions-runner-controller/api/v1alpha1"
)

// SetupTest will set up a testing environment.
// This includes:
// * creating a Namespace to be used during the test
// * starting the 'RunnerReconciler'
// * stopping the 'RunnerSetReconciler" after the test ends
// Call this function at the start of each of your tests.
func SetupTest(ctx context.Context) *corev1.Namespace {
	var stopCh chan struct{}
	ns := &corev1.Namespace{}

	BeforeEach(func() {
		stopCh = make(chan struct{})
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "testns-" + randStringRunes(5)},
		}

		err := k8sClient.Create(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		controller := &RunnerSetReconciler{
			Client:   mgr.GetClient(),
			Scheme:   scheme.Scheme,
			Log:      logf.Log,
			Recorder: mgr.GetEventRecorderFor("runnerset-controller"),
		}
		err = controller.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			defer GinkgoRecover()

			err := mgr.Start(stopCh)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()
	})

	AfterEach(func() {
		close(stopCh)

		err := k8sClient.Delete(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test namespace")
	})

	return ns
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func intPtr(v int) *int {
	return &v
}

var _ = Context("Inside of a new namespace", func() {
	ctx := context.TODO()
	ns := SetupTest(ctx)

	Describe("when no existing resources exist", func() {

		It("should create a new Runner resource from the specified template, add a another Runner on replicas increased, and removes all the replicas when set to 0", func() {
			name := "example-runnerset"

			{
				rs := &actionsv1alpha1.RunnerSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns.Name,
					},
					Spec: actionsv1alpha1.RunnerSetSpec{
						Replicas: intPtr(1),
						Template: actionsv1alpha1.RunnerTemplate{
							Spec: actionsv1alpha1.RunnerSpec{
								Repository: "foo/bar",
								Image:      "bar",
								Env: []corev1.EnvVar{
									{Name: "FOO", Value: "FOOVALUE"},
								},
							},
						},
					},
				}

				err := k8sClient.Create(ctx, rs)

				Expect(err).NotTo(HaveOccurred(), "failed to create test RunnerSet resource")

				runners := actionsv1alpha1.RunnerList{Items: []actionsv1alpha1.Runner{}}

				Eventually(
					func() int {
						err := k8sClient.List(ctx, &runners, client.InNamespace(ns.Name))
						if err != nil {
							logf.Log.Error(err, "list runners")
						}

						return len(runners.Items)
					},
					time.Second*5, time.Millisecond*500).Should(BeEquivalentTo(1))
			}

			{
				// We wrap the update in the Eventually block to avoid the below error that occurs due to concurrent modification
				// made by the controller to update .Status.AvailableReplicas and .Status.ReadyReplicas
				//   Operation cannot be fulfilled on runnersets.actions.summerwind.dev "example-runnerset": the object has been modified; please apply your changes to the latest version and try again
				Eventually(func() error {
					var rs actionsv1alpha1.RunnerSet

					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns.Name, Name: name}, &rs)

					Expect(err).NotTo(HaveOccurred(), "failed to get test RunnerSet resource")

					rs.Spec.Replicas = intPtr(2)

					return k8sClient.Update(ctx, &rs)
				},
					time.Second*1, time.Millisecond*500).Should(BeNil())

				runners := actionsv1alpha1.RunnerList{Items: []actionsv1alpha1.Runner{}}

				Eventually(
					func() int {
						err := k8sClient.List(ctx, &runners, client.InNamespace(ns.Name))
						if err != nil {
							logf.Log.Error(err, "list runners")
						}

						return len(runners.Items)
					},
					time.Second*5, time.Millisecond*500).Should(BeEquivalentTo(2))
			}

			{
				// We wrap the update in the Eventually block to avoid the below error that occurs due to concurrent modification
				// made by the controller to update .Status.AvailableReplicas and .Status.ReadyReplicas
				//   Operation cannot be fulfilled on runnersets.actions.summerwind.dev "example-runnerset": the object has been modified; please apply your changes to the latest version and try again
				Eventually(func() error {
					var rs actionsv1alpha1.RunnerSet

					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: ns.Name, Name: name}, &rs)

					Expect(err).NotTo(HaveOccurred(), "failed to get test RunnerSet resource")

					rs.Spec.Replicas = intPtr(0)

					return k8sClient.Update(ctx, &rs)
				},
					time.Second*1, time.Millisecond*500).Should(BeNil())

				runners := actionsv1alpha1.RunnerList{Items: []actionsv1alpha1.Runner{}}

				Eventually(
					func() int {
						err := k8sClient.List(ctx, &runners, client.InNamespace(ns.Name))
						if err != nil {
							logf.Log.Error(err, "list runners")
						}

						return len(runners.Items)
					},
					time.Second*5, time.Millisecond*500).Should(BeEquivalentTo(0))
			}
		})
	})
})
