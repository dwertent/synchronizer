package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

var (
	configmap       = unstructured.Unstructured{Object: map[string]interface{}{"kind": "ConfigMap", "metadata": map[string]interface{}{"uid": "748ad4a8-e5ff-44da-ba94-309992c97820"}}}
	deployment      = unstructured.Unstructured{Object: map[string]interface{}{"kind": "Deployment", "metadata": map[string]interface{}{"uid": "6b1a0c50-277f-4aa1-a4f9-9fc278ce4fe2"}}}
	pod             = unstructured.Unstructured{Object: map[string]interface{}{"kind": "Pod", "metadata": map[string]interface{}{"uid": "aa5e3e8f-2da5-4c38-93c0-210d3280d10f"}}}
	deploymentAdded = watch.Event{Type: watch.Added, Object: &deployment}
	podAdded        = watch.Event{Type: watch.Added, Object: &pod}
	podModified     = watch.Event{Type: watch.Modified, Object: &pod}
)

func TestCooldownQueue_Enqueue(t *testing.T) {
	tests := []struct {
		name      string
		inEvents  []watch.Event
		outEvents []watch.Event
	}{
		{
			name:      "add pod",
			inEvents:  []watch.Event{deploymentAdded, podAdded, podModified, podModified, podModified},
			outEvents: []watch.Event{deploymentAdded, podAdded, podModified},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewCooldownQueue(DefaultQueueSize, 1*time.Second)
			go func() {
				time.Sleep(2 * time.Second)
				q.Stop()
			}()
			for _, e := range tt.inEvents {
				time.Sleep(50 * time.Millisecond) // need to sleep to preserve order since the insertion is async
				q.Enqueue(e)
			}
			outEvents := []watch.Event{}
			for e := range q.ResultChan {
				outEvents = append(outEvents, e)
			}
			assert.Equal(t, tt.outEvents, outEvents)
		})
	}
}

func Test_makeEventKey(t *testing.T) {
	tests := []struct {
		name string
		e    watch.Event
		want string
	}{
		{
			name: "add pod",
			e: watch.Event{
				Type:   watch.Added,
				Object: &pod,
			},
			want: "ADDED-aa5e3e8f-2da5-4c38-93c0-210d3280d10f",
		},
		{
			name: "delete deployment",
			e: watch.Event{
				Type:   watch.Deleted,
				Object: &deployment,
			},
			want: "DELETED-6b1a0c50-277f-4aa1-a4f9-9fc278ce4fe2",
		},
		{
			name: "modify configmap",
			e: watch.Event{
				Type:   watch.Modified,
				Object: &configmap,
			},
			want: "MODIFIED-748ad4a8-e5ff-44da-ba94-309992c97820",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeEventKey(tt.e)
			assert.Equal(t, tt.want, got)
		})
	}
}
