package proxy

import (
	"testing"

	"centag/core/pkg/pipeline"
)

func TestPipelineCacheHit(t *testing.T) {
	if pipelineCacheHit(nil) {
		t.Fatal("nil output should not be cache hit")
	}
	if pipelineCacheHit(&pipeline.PipelineOutput{}) {
		t.Fatal("empty output should not be cache hit")
	}
	if !pipelineCacheHit(&pipeline.PipelineOutput{
		Metadata: map[string]interface{}{"cache_hit": true},
	}) {
		t.Fatal("expected cache_hit=true")
	}
}

func TestCacheLayerFromOutput(t *testing.T) {
	if cacheLayerFromOutput(nil) != "L1" {
		t.Fatal("nil -> L1")
	}
	if cacheLayerFromOutput(&pipeline.PipelineOutput{
		Metadata: map[string]interface{}{"strategy": "semantic", "cache_hit": true},
	}) != "L2" {
		t.Fatal("semantic -> L2")
	}
	if cacheLayerFromOutput(&pipeline.PipelineOutput{
		Metadata: map[string]interface{}{"cache_score": 0.91, "cache_hit": true},
	}) != "L2" {
		t.Fatal("cache_score -> L2")
	}
}