//! iOS C-ABI surface for the chat-openai capability.
//!
//! Mirrors the Android JNI surface. Differences:
//!
//! * Strings cross as `*mut c_char` (UTF-8, null-terminated).
//! * Every fallible function returns a `*mut c_char` containing JSON.
//!   On success: `{"ok":true,"data":<value>}`. On failure:
//!   `{"ok":false,"error":{"code":"...","message":"..."}}`. Callers
//!   must `llmhub_string_free` whatever they got back, including the
//!   error JSON.
//! * Streaming calls take three `extern "C"` callbacks + a `user_data`
//!   pointer. The pointer is opaque to Rust; we hand it back as-is.
//!
//! Threading: all calls block the caller's thread. The Swift wrapper
//! must dispatch to a background queue.
#![allow(non_snake_case)]

extern crate alloc;

use std::ffi::{c_char, c_void, CStr, CString};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::ptr;
use std::sync::Arc;

use llmhub_chat_openai::{ChatClient, ChatCompletionRequest};
use llmhub_core::{Error, PlatformClient};
use serde_json::json;

// ---------------------------------------------------------------------
// Handle plumbing
// ---------------------------------------------------------------------

fn into_handle<T>(v: T) -> *mut c_void {
    Box::into_raw(Box::new(v)) as *mut c_void
}

unsafe fn handle_ref<'a, T>(p: *mut c_void) -> Option<&'a T> {
    if p.is_null() { None } else { Some(&*(p as *const T)) }
}

unsafe fn drop_handle<T>(p: *mut c_void) {
    if !p.is_null() {
        drop(Box::from_raw(p as *mut T));
    }
}

// ---------------------------------------------------------------------
// String / result plumbing
// ---------------------------------------------------------------------

/// Marshal a Rust `String` into a heap C string the caller must free.
fn into_c_string(s: String) -> *mut c_char {
    match CString::new(s) {
        Ok(c) => c.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

/// Unwrap a `*const c_char` argument. Returns `None` on null.
unsafe fn from_c_str<'a>(p: *const c_char) -> Option<&'a str> {
    if p.is_null() { return None; }
    CStr::from_ptr(p).to_str().ok()
}

fn ok_envelope(data_json: serde_json::Value) -> *mut c_char {
    into_c_string(json!({ "ok": true, "data": data_json }).to_string())
}

fn err_envelope(code: &str, message: &str) -> *mut c_char {
    into_c_string(
        json!({
            "ok": false,
            "error": { "code": code, "message": message }
        })
        .to_string(),
    )
}

fn err_from(e: &Error) -> *mut c_char {
    err_envelope(e.code(), &e.to_string())
}

/// Free a string previously returned by an `llmhub_*` function.
/// Safe to call with NULL.
#[no_mangle]
pub unsafe extern "C" fn llmhub_string_free(s: *mut c_char) {
    if !s.is_null() {
        drop(CString::from_raw(s));
    }
}

// ---------------------------------------------------------------------
// PlatformClient lifecycle
// ---------------------------------------------------------------------

#[no_mangle]
pub unsafe extern "C" fn llmhub_platform_new(
    base_url: *const c_char,
    api_key: *const c_char,
) -> *mut c_void {
    catch_unwind(AssertUnwindSafe(|| {
        let base = match from_c_str(base_url) { Some(s) => s.to_string(), None => return ptr::null_mut() };
        let key  = match from_c_str(api_key)  { Some(s) => s.to_string(), None => return ptr::null_mut() };
        match PlatformClient::new(base, key) {
            Ok(c)  => into_handle::<Arc<PlatformClient>>(Arc::new(c)),
            Err(_) => ptr::null_mut(),
        }
    })).unwrap_or(ptr::null_mut())
}

#[no_mangle]
pub unsafe extern "C" fn llmhub_platform_free(handle: *mut c_void) {
    drop_handle::<Arc<PlatformClient>>(handle)
}

#[no_mangle]
pub unsafe extern "C" fn llmhub_platform_list_services(handle: *mut c_void) -> *mut c_char {
    catch_unwind(AssertUnwindSafe(|| {
        let platform = match handle_ref::<Arc<PlatformClient>>(handle) {
            Some(p) => p,
            None    => return err_envelope("invalid_handle", "platform handle is null"),
        };
        match platform.list_services() {
            Ok(svcs) => {
                let v = serde_json::to_value(svcs).unwrap_or(json!([]));
                ok_envelope(v)
            }
            Err(e) => err_from(&e),
        }
    })).unwrap_or_else(|_| err_envelope("panic", "panic in llmhub_platform_list_services"))
}

// ---------------------------------------------------------------------
// ChatClient lifecycle
// ---------------------------------------------------------------------

#[no_mangle]
pub unsafe extern "C" fn llmhub_chat_new(
    platform: *mut c_void,
    sku_id: *const c_char,
) -> *mut c_void {
    catch_unwind(AssertUnwindSafe(|| {
        let platform = match handle_ref::<Arc<PlatformClient>>(platform) {
            Some(p) => p,
            None    => return ptr::null_mut(),
        };
        let sku = match from_c_str(sku_id) {
            Some(s) => s.to_string(),
            None    => return ptr::null_mut(),
        };
        into_handle::<ChatClient>(ChatClient::new(platform.clone(), sku))
    })).unwrap_or(ptr::null_mut())
}

#[no_mangle]
pub unsafe extern "C" fn llmhub_chat_free(handle: *mut c_void) {
    drop_handle::<ChatClient>(handle)
}

// ---------------------------------------------------------------------
// Chat completions
// ---------------------------------------------------------------------

#[no_mangle]
pub unsafe extern "C" fn llmhub_chat_create(
    chat: *mut c_void,
    request_json: *const c_char,
) -> *mut c_char {
    catch_unwind(AssertUnwindSafe(|| {
        let chat = match handle_ref::<ChatClient>(chat) {
            Some(c) => c,
            None    => return err_envelope("invalid_handle", "chat handle is null"),
        };
        let body = match from_c_str(request_json) {
            Some(s) => s,
            None    => return err_envelope("invalid_argument", "request_json is null"),
        };
        let req: ChatCompletionRequest = match serde_json::from_str(body) {
            Ok(r) => r,
            Err(e) => return err_envelope("decode_error", &e.to_string()),
        };
        match chat.create(req) {
            Ok(resp) => {
                let v = serde_json::to_value(resp).unwrap_or(json!({}));
                ok_envelope(v)
            }
            Err(e) => err_from(&e),
        }
    })).unwrap_or_else(|_| err_envelope("panic", "panic in llmhub_chat_create"))
}

pub type OnChunkFn   = extern "C" fn(user_data: *mut c_void, chunk_json: *const c_char) -> bool;
pub type OnCompleteFn = extern "C" fn(user_data: *mut c_void);
pub type OnErrorFn    = extern "C" fn(user_data: *mut c_void, code: *const c_char, message: *const c_char);

/// Streaming completion. Pumps each SSE chunk through `on_chunk(json)`
/// and finishes with `on_complete()` or `on_error(code, message)`.
/// `on_chunk` returning `false` cancels the stream.
///
/// `user_data` is opaque — usually a retained Swift closure box.
#[no_mangle]
pub unsafe extern "C" fn llmhub_chat_create_stream(
    chat: *mut c_void,
    request_json: *const c_char,
    user_data: *mut c_void,
    on_chunk: OnChunkFn,
    on_complete: OnCompleteFn,
    on_error: OnErrorFn,
) {
    let r = catch_unwind(AssertUnwindSafe(|| -> Result<(), Error> {
        let chat = handle_ref::<ChatClient>(chat).ok_or_else(|| Error::Platform {
            code: "invalid_handle".into(),
            message: "chat handle is null".into(),
        })?;
        let body = from_c_str(request_json).ok_or_else(|| Error::Decode("request_json is null".into()))?;
        let mut req: ChatCompletionRequest =
            serde_json::from_str(body).map_err(|e| Error::Decode(e.to_string()))?;
        req.stream = true;

        chat.create_stream(req, &mut |chunk| {
            let json = match serde_json::to_string(&chunk) {
                Ok(s) => s,
                Err(_) => return false,
            };
            let cs = match CString::new(json) { Ok(c) => c, Err(_) => return false };
            on_chunk(user_data, cs.as_ptr())
        })?;
        Ok(())
    }));

    match r {
        Ok(Ok(()))  => on_complete(user_data),
        Ok(Err(e))  => {
            let code = CString::new(e.code()).unwrap_or_default();
            let msg  = CString::new(e.to_string()).unwrap_or_default();
            on_error(user_data, code.as_ptr(), msg.as_ptr());
        }
        Err(_) => {
            let code = CString::new("panic").unwrap();
            let msg  = CString::new("panic in llmhub_chat_create_stream").unwrap();
            on_error(user_data, code.as_ptr(), msg.as_ptr());
        }
    }
}

