package test

import "testing"

func TestCachedGC(t *testing.T) {
	for _, builder := range cachedGCTableBuilders {
		t.Run(builder.name, func(t *testing.T) {
			cachedGarbageCollectionTest(t, builder)
		})
	}
}

func TestPurgeMultivolume(t *testing.T) {
	purgeLocksTest(t)
}
