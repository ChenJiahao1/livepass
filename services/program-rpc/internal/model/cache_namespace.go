package model

import "strings"

const (
	defaultCacheDProgramIdPrefix               = "cache:dProgram:id:"
	defaultCacheDProgramGroupIdPrefix          = "cache:dProgramGroup:id:"
	defaultCacheDProgramShowTimeIdPrefix       = "cache:dProgramShowTime:id:"
	defaultCacheDProgramShowTimeFirstProgramID = "cache:dProgramShowTime:first:programId:"
)

var cacheDProgramShowTimeFirstProgramIDPrefix = defaultCacheDProgramShowTimeFirstProgramID

func SetCacheKeyNamespace(namespace string) {
	cacheDProgramIdPrefix = namespacedCachePrefix(namespace, defaultCacheDProgramIdPrefix)
	cacheDProgramGroupIdPrefix = namespacedCachePrefix(namespace, defaultCacheDProgramGroupIdPrefix)
	cacheDProgramShowTimeIdPrefix = namespacedCachePrefix(namespace, defaultCacheDProgramShowTimeIdPrefix)
	cacheDProgramShowTimeFirstProgramIDPrefix = namespacedCachePrefix(namespace, defaultCacheDProgramShowTimeFirstProgramID)
}

func ProgramCacheKeyPrefix() string {
	return cacheDProgramIdPrefix
}

func ProgramGroupCacheKeyPrefix() string {
	return cacheDProgramGroupIdPrefix
}

func ProgramShowTimeCacheKeyPrefix() string {
	return cacheDProgramShowTimeIdPrefix
}

func ProgramFirstShowTimeCacheKeyPrefix() string {
	return cacheDProgramShowTimeFirstProgramIDPrefix
}

func namespacedCachePrefix(namespace, base string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return base
	}

	namespace = strings.Trim(namespace, ":")
	return namespace + ":" + base
}
