//go:build darwin

package backend

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework WebKit -lobjc
#import <objc/runtime.h>
#import <WebKit/WebKit.h>

static BOOL alwaysAcceptsFirstMouse(id self, SEL _cmd, NSEvent *event) {
	return YES;
}

static void enableClickThrough(void) {
	Method m = class_getInstanceMethod([WKWebView class], @selector(acceptsFirstMouse:));
	method_setImplementation(m, (IMP)alwaysAcceptsFirstMouse);
}
*/
import "C"

func init() {
	C.enableClickThrough()
}
