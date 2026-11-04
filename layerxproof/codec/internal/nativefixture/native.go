// Package nativefixture runs the production C witness codecs over committed
// fixture state without starting a compiler or a producer subprocess.
package nativefixture

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../include -I${SRCDIR}/../../../../build/generated
#cgo LDFLAGS: -L${SRCDIR}/../../../../build -L${SRCDIR}/../../../../programs/target/release -L${SRCDIR}/../../../../programs/target/debug -llayerx -llayerx_programs_sandbox -llayerx -lssl -lcrypto -pthread -ldl -lm
#include <stdio.h>

static FILE *fixture_output;
#define printf(...) fprintf(fixture_output, __VA_ARGS__)
#define putchar(c) fputc((c), fixture_output)
#define main current_host_fixture
#include "../../testdata/repair_1.c"
#undef main
#include "../../testdata/repair_m1_compat.c"
#undef printf
#undef putchar

static char *fixture_document(int compatibility, size_t *length)
{
    char *data = NULL;
    fixture_output = open_memstream(&data, length);
    if (fixture_output == NULL) return NULL;
    if (compatibility) compatibility_fixture();
    else current_host_fixture();
    if (fclose(fixture_output) != 0) {
        free(data);
        return NULL;
    }
    return data;
}
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

var fixtureMu sync.Mutex

// Document returns encoded witnesses from the current host, or from the
// retained twelve-leaf codec fixture when compatibility is true.
func Document(compatibility bool) ([]byte, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	mode := C.int(0)
	if compatibility {
		mode = 1
	}
	var length C.size_t
	data := C.fixture_document(mode, &length)
	if data == nil {
		return nil, errors.New("native fixture output failed")
	}
	defer C.free(unsafe.Pointer(data))
	return C.GoBytes(unsafe.Pointer(data), C.int(length)), nil
}
