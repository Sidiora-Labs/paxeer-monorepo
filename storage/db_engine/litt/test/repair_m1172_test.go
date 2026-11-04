package test

import "testing"

func TestRepairM_1172_CachedGC(t *testing.T) {
	for _, builder := range cachedGCTableBuilders {
		t.Run(builder.name, func(t *testing.T) {
			cachedGarbageCollectionTest(t, builder)
		})
	}
}

func TestRepairM_1172_PurgeMultivolume(t *testing.T) {
	purgeLocksTest(t)
}
