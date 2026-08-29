package disk

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

// availableForImportantUsage answers what macOS itself calls available, which counts the
// purgeable space the system reclaims on its own (ADR 0014). It returns 0 on success.
static int availableForImportantUsage(const char *path, long long *out) {
	CFStringRef text = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
	if (text == NULL) {
		return -1;
	}
	CFURLRef url = CFURLCreateWithFileSystemPath(NULL, text, kCFURLPOSIXPathStyle, true);
	CFRelease(text);
	if (url == NULL) {
		return -1;
	}

	CFNumberRef value = NULL;
	CFErrorRef failure = NULL;
	Boolean answered = CFURLCopyResourcePropertyForKey(
		url, kCFURLVolumeAvailableCapacityForImportantUsageKey, &value, &failure);
	CFRelease(url);
	if (!answered || value == NULL) {
		if (failure != NULL) {
			CFRelease(failure);
		}
		if (value != NULL) {
			CFRelease(value);
		}
		return -1;
	}

	long long bytes = 0;
	Boolean read = CFNumberGetValue(value, kCFNumberLongLongType, &bytes);
	CFRelease(value);
	if (!read || bytes < 0) {
		return -1;
	}
	*out = bytes;
	return 0;
}
*/
import "C"

import "unsafe"

// available asks macOS how much space the volume has for important use. The second result
// is false when the system declines to answer, and the caller falls back to statfs.
//
// Zero counts as declining: macOS answers zero for volumes that may not hold important
// data at all — the system volumes and a Time Machine target — where statfs still reports
// the real headroom. A volume genuinely at zero reads the same either way.
func available(mount string) (uint64, bool) {
	path := C.CString(mount)
	defer C.free(unsafe.Pointer(path))

	var bytes C.longlong
	if C.availableForImportantUsage(path, &bytes) != 0 || bytes == 0 {
		return 0, false
	}
	return uint64(bytes), true
}
