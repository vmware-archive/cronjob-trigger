package controller

import (
	"testing"
	"time"

	cronjobtriggerapi "github.com/kubeless/cronjob-trigger/pkg/apis/kubeless/v1beta1"
	cronjobTriggerFake "github.com/kubeless/cronjob-trigger/pkg/client/clientset/versioned/fake"
	kubelessApi "github.com/kubeless/kubeless/pkg/apis/kubeless/v1beta1"
	"github.com/sirupsen/logrus"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFunctionAddedUpdated(t *testing.T) {
	myNsFoo := metav1.ObjectMeta{
		Namespace: "myns",
		Name:      "foo",
	}

	f := kubelessApi.Function{
		ObjectMeta: myNsFoo,
	}

	cjtrigger := cronjobtriggerapi.CronJobTrigger{
		ObjectMeta: myNsFoo,
	}

	triggerClientset := cronjobTriggerFake.NewSimpleClientset(&cjtrigger)

	cronjob := batchv1beta1.CronJob{
		ObjectMeta: myNsFoo,
	}
	clientset := fake.NewSimpleClientset(&cronjob)

	controller := CronJobTriggerController{
		clientset:     clientset,
		cronjobclient: triggerClientset,
		logger:        logrus.WithField("controller", "cronjob-trigger-controller"),
	}

	// no-op for when the function is not deleted
	err := controller.functionAddedDeletedUpdated(&f, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	list, err := controller.cronjobclient.KubelessV1beta1().CronJobTriggers("myns").List(metav1.ListOptions{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ObjectMeta.Name != "foo" {
		t.Errorf("Missing trigger in list: %v", list.Items)
	}
}

func TestFunctionDeleted(t *testing.T) {
	myNsFoo := metav1.ObjectMeta{
		Namespace: "myns",
		Name:      "foo",
	}

	f := kubelessApi.Function{
		ObjectMeta: myNsFoo,
	}

	cjtrigger := cronjobtriggerapi.CronJobTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "myns",
			Name:      "foo-trigger",
		},
		Spec: cronjobtriggerapi.CronJobTriggerSpec{
			FunctionName: "foo",
		},
	}

	triggerClientset := cronjobTriggerFake.NewSimpleClientset(&cjtrigger)

	cronjob := batchv1beta1.CronJob{
		ObjectMeta: myNsFoo,
	}
	clientset := fake.NewSimpleClientset(&cronjob)

	controller := CronJobTriggerController{
		clientset:     clientset,
		cronjobclient: triggerClientset,
		logger:        logrus.WithField("controller", "cronjob-trigger-controller"),
	}

	// no-op for when the function is not deleted
	err := controller.functionAddedDeletedUpdated(&f, true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	list, err := controller.cronjobclient.KubelessV1beta1().CronJobTriggers("myns").List(metav1.ListOptions{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(list.Items) != 0 {
		t.Errorf("Trigger should be deleted from list: %v", list.Items)
	}
}

func TestCronJobTriggerObjChanged(t *testing.T) {
	type testObj struct {
		old             *cronjobtriggerapi.CronJobTrigger
		new             *cronjobtriggerapi.CronJobTrigger
		expectedChanged bool
	}
	t1 := metav1.Time{
		Time: time.Now(),
	}
	t2 := metav1.Time{
		Time: time.Now(),
	}
	testObjs := []testObj{
		{
			old:             &cronjobtriggerapi.CronJobTrigger{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			new:             &cronjobtriggerapi.CronJobTrigger{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			expectedChanged: false,
		},
		{
			old:             &cronjobtriggerapi.CronJobTrigger{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &t1}},
			new:             &cronjobtriggerapi.CronJobTrigger{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &t2}},
			expectedChanged: true,
		},
		{
			old:             &cronjobtriggerapi.CronJobTrigger{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}},
			new:             &cronjobtriggerapi.CronJobTrigger{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}},
			expectedChanged: true,
		},
		{
			old:             &cronjobtriggerapi.CronJobTrigger{Spec: cronjobtriggerapi.CronJobTriggerSpec{Schedule: "* * * * *"}},
			new:             &cronjobtriggerapi.CronJobTrigger{Spec: cronjobtriggerapi.CronJobTriggerSpec{Schedule: "* * * * *"}},
			expectedChanged: false,
		},
		{
			old:             &cronjobtriggerapi.CronJobTrigger{Spec: cronjobtriggerapi.CronJobTriggerSpec{Schedule: "*/10 * * * *"}},
			new:             &cronjobtriggerapi.CronJobTrigger{Spec: cronjobtriggerapi.CronJobTriggerSpec{Schedule: "* * * * *"}},
			expectedChanged: true,
		},
	}
	for _, to := range testObjs {
		changed := cronJobTriggerObjChanged(to.old, to.new)
		if changed != to.expectedChanged {
			t.Errorf("%v != %v expected to be %v", to.old, to.new, to.expectedChanged)
		}
	}
}
