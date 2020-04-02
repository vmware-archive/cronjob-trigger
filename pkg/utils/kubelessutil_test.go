package utils

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	cronjobTriggerApi "github.com/kubeless/cronjob-trigger/pkg/apis/kubeless/v1beta1"
	kubelessApi "github.com/kubeless/kubeless/pkg/apis/kubeless/v1beta1"

	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureCronJob(t *testing.T) {
	or := []metav1.OwnerReference{
		{
			Kind:       "Function",
			APIVersion: "kubeless.io/v1beta1",
		},
	}
	ns := "default"
	f1Name := "func1"
	newSchedule := "* * * * *"
	f1 := &kubelessApi.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f1Name,
			Namespace: ns,
			Labels: map[string]string{
				"test":    "true",
				"only-fn": "ok",
			},
			Annotations: map[string]string{
				"kubeless.io/test":          "test",
				"kubeless.io/function-only": "this should exist",
			},
		},
		Spec: kubelessApi.FunctionSpec{
			Timeout: "120",
		},
	}
	cronjobTriggerObj := &cronjobTriggerApi.CronJobTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"test": "false",
			},
			Annotations: map[string]string{
				"kubeless.io/test": "not a test",
				"kubeless.io/new":  "new",
			},
		},
		Spec: cronjobTriggerApi.CronJobTriggerSpec{
			Schedule: newSchedule,
		},
	}
	expectedMeta := metav1.ObjectMeta{
		Name:            "trigger-" + f1Name,
		Namespace:       ns,
		OwnerReferences: or,
		Labels: map[string]string{
			"test":       "false",
			"only-fn":    "ok",
			"created-by": "kubeless",
		},
		Annotations: map[string]string{
			"kubeless.io/test":          "not a test",
			"kubeless.io/function-only": "this should exist",
			"kubeless.io/new":           "new",
		},
	}

	clientset := fake.NewSimpleClientset()

	pullSecrets := []v1.LocalObjectReference{
		{Name: "creds"},
	}
	err := EnsureCronJob(clientset, f1, cronjobTriggerObj, "unzip", or, pullSecrets)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	cronJob, err := clientset.BatchV1beta1().CronJobs(ns).Get(fmt.Sprintf("trigger-%s", f1.Name), metav1.GetOptions{})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if !reflect.DeepEqual(expectedMeta, cronJob.ObjectMeta) {
		t.Errorf("Unexpected metadata metadata. Expecting\n%+v \nReceived:\n%+v", expectedMeta, cronJob.ObjectMeta)
	}
	if *cronJob.Spec.SuccessfulJobsHistoryLimit != int32(3) {
		t.Errorf("Unexpected SuccessfulJobsHistoryLimit: %d", *cronJob.Spec.SuccessfulJobsHistoryLimit)
	}
	if *cronJob.Spec.FailedJobsHistoryLimit != int32(1) {
		t.Errorf("Unexpected FailedJobsHistoryLimit: %d", *cronJob.Spec.FailedJobsHistoryLimit)
	}
	if *cronJob.Spec.JobTemplate.Spec.ActiveDeadlineSeconds != int64(120) {
		t.Errorf("Unexpected ActiveDeadlineSeconds: %d", *cronJob.Spec.JobTemplate.Spec.ActiveDeadlineSeconds)
	}

	expectedId := "\"Event-Id: $(POD_UID)\""
	expectedTime := "\"Event-Time: $(date --rfc-3339=seconds --utc)\""
	expectedNamespace := "\"Event-Namespace: cronjobtrigger.kubeless.io\""
	expectedType := "\"Event-Type: application/json\""
	contentType := "\"Content-Type: application/json\""

	expectedHeaders := fmt.Sprintf("-H %s -H %s -H %s -H %s -H %s", expectedId, expectedTime, expectedNamespace, expectedType, contentType)
	expectedEndpoint := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", f1Name, ns)
	expectedCommand := fmt.Sprintf("curl -Lv %s %s", expectedHeaders, expectedEndpoint)

	runtimeContainer := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if runtimeContainer.Image != "unzip" {
		t.Errorf("Unexpected image %s", runtimeContainer.Image)
	}
	args := runtimeContainer.Args
	// skip event headers data (i.e  -H "event-id: cronjob-controller-2018-03-05T05:55:41.990784027Z" etc)
	foundCommand := args[0]
	if !reflect.DeepEqual(foundCommand, expectedCommand) {
		t.Errorf("Unexpected command %s expexted %s", foundCommand, expectedCommand)
	}

	newSchedule = "*/10 * * * *"
	newData := make(map[string]string)
	newData["test"] = "foo"

	cronjobTriggerObj.Spec.Schedule = newSchedule
	cronjobTriggerObj.Spec.Payload = newData

	err = EnsureCronJob(clientset, f1, cronjobTriggerObj, "unzip", or, pullSecrets)
	cronJob, err = clientset.BatchV1beta1().CronJobs(ns).Get(fmt.Sprintf("trigger-%s", f1.Name), metav1.GetOptions{})

	runtimeContainer = cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	args = runtimeContainer.Args
	foundCommand = args[0]

	expectedData := "-d '{\"test\":\"foo\"}'"
	expectedCommand = fmt.Sprintf("curl -Lv %s %s %s", expectedHeaders, expectedEndpoint, expectedData)

	// if !reflect.DeepEqual(foundCommand, expectedCommand) {
	// 	t.Errorf("Unexpected command %s expected %s", foundCommand, expectedCommand)
	// }

	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	updatedCronJob, err := clientset.BatchV1beta1().CronJobs(ns).Get(fmt.Sprintf("trigger-%s", f1.Name), metav1.GetOptions{})
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if updatedCronJob.Spec.Schedule != newSchedule {
		t.Errorf("Unexpected schedule %s expecting %s", updatedCronJob.Spec.Schedule, newSchedule)
	}
}

func TestAvoidCronjobOverwrite(t *testing.T) {
	or := []metav1.OwnerReference{}
	ns := "default"
	f1Name := "func1"
	newSchedule := "* * * * *"
	f1 := &kubelessApi.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f1Name,
			Namespace: ns,
		},
		Spec: kubelessApi.FunctionSpec{},
	}
	cronjobTriggerObj := &cronjobTriggerApi.CronJobTrigger{
		Spec: cronjobTriggerApi.CronJobTriggerSpec{
			Schedule: newSchedule,
		},
	}

	clientset := fake.NewSimpleClientset()

	clientset.BatchV1beta1().CronJobs(ns).Create(&batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("trigger-%s", f1.Name)},
	})
	err := EnsureCronJob(clientset, f1, cronjobTriggerObj, "unzip", or, []v1.LocalObjectReference{})
	if err == nil && strings.Contains(err.Error(), "conflicting object") {
		t.Errorf("It should fail because a conflict")
	}
}

func TestMergeMaps(t *testing.T) {
	fnMap := map[string]string{
		"fnOverwritten": "nok",
		"fnUntouched":   "ok",
		"tgUntouched":   "nok",
	}
	tgMap := map[string]string{
		"fnOverwritten": "ok",
		"tgUntouched":   "ok",
	}
	expected := map[string]string{
		"fnOverwritten": "ok",
		"fnUntouched":   "ok",
		"tgUntouched":   "ok",
	}

	result := mergeMaps(tgMap, fnMap)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Unexpected merged result: \n Expecting: \n %s \n Received: \n %s", expected, result)
	}
}
