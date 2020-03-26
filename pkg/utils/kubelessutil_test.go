package utils

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

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
	f1 := &kubelessApi.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f1Name,
			Namespace: ns,
		},
		Spec: kubelessApi.FunctionSpec{
			Timeout: "120",
		},
	}
	expectedMeta := metav1.ObjectMeta{
		Name:            "trigger-" + f1Name,
		Namespace:       ns,
		OwnerReferences: or,
		Labels:          addDefaultLabel(map[string]string{}),
	}

	clientset := fake.NewSimpleClientset()

	pullSecrets := []v1.LocalObjectReference{
		{Name: "creds"},
	}
	err := EnsureCronJob(clientset, f1, "* * * * *", "unzip", or, pullSecrets)
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

	expectedId := "\"event-id: $(POD_UID)\""
	expectedTime := "\"event-time: $(date --rfc-3339=seconds --utc)\""
	expectedType := "\"event-type: application/json\""
	expectedNamespace := "\"event-namespace: cronjobtrigger.kubeless.io\""

	expectedHeaders := fmt.Sprintf("-H %s -H %s -H %s -H %s", expectedId, expectedTime, expectedType, expectedNamespace)
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

	// It should update the existing cronJob if it is already created
	newSchedule := "*/10 * * * *"
	err = EnsureCronJob(clientset, f1, newSchedule, "unzip", or, pullSecrets)
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
	f1 := &kubelessApi.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f1Name,
			Namespace: ns,
		},
		Spec: kubelessApi.FunctionSpec{},
	}
	clientset := fake.NewSimpleClientset()

	clientset.BatchV1beta1().CronJobs(ns).Create(&batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("trigger-%s", f1.Name)},
	})
	err := EnsureCronJob(clientset, f1, "* * * * *", "unzip", or, []v1.LocalObjectReference{})
	if err == nil && strings.Contains(err.Error(), "conflicting object") {
		t.Errorf("It should fail because a conflict")
	}
}
