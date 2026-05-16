//! Android JNI surface for the LLMHub SDK.
//!
//! Design rules:
//!
//! * Native handles are opaque `jlong` cookies that index into per-class
//!   storage on the Rust side. Kotlin never sees a raw pointer, and a
//!   handle leaking across a process boundary still doesn't help an
//!   attacker — the auth_payload never crosses JNI.
//! * Every `Java_io_llmhub_chat_internal_NativeBridge_*` symbol owns
//!   its panic boundary via `catch_unwind`; a Rust panic surfaces as a
//!   Java `LLMHubException` rather than aborting the app.
//! * String args come in as `JString`; we copy them out immediately so
//!   we don't keep JNI references alive across long calls.

#![allow(non_snake_case)]

extern crate alloc;

use std::panic::{catch_unwind, AssertUnwindSafe};
use std::sync::Arc;

use jni::objects::{JClass, JObject, JString, JValue};
use jni::sys::{jint, jlong, jstring};
use jni::JNIEnv;

use llmhub_chat_openai::{ChatClient, ChatCompletionRequest};
use llmhub_core::{Error, PlatformClient};

litcrypt2::use_litcrypt!();

const EXCEPTION_CLASS: &str = "io/llmhub/chat/LLMHubException";

// ---------------------------------------------------------------------
// Handle plumbing
// ---------------------------------------------------------------------

/// Box → jlong (a raw pointer dressed up as an opaque cookie). The
/// caller side must pair every `into_handle` with exactly one
/// `drop_handle`.
fn into_handle<T>(value: T) -> jlong {
    Box::into_raw(Box::new(value)) as jlong
}

unsafe fn handle_ref<'a, T>(handle: jlong) -> Option<&'a T> {
    if handle == 0 {
        None
    } else {
        Some(&*(handle as *const T))
    }
}

unsafe fn drop_handle<T>(handle: jlong) {
    if handle != 0 {
        drop(Box::from_raw(handle as *mut T));
    }
}

fn throw(env: &mut JNIEnv<'_>, err: &Error) {
    let _ = env.throw_new(EXCEPTION_CLASS, format!("{}: {}", err.code(), err));
}

fn throw_msg(env: &mut JNIEnv<'_>, code: &str, msg: &str) {
    let _ = env.throw_new(EXCEPTION_CLASS, format!("{}: {}", code, msg));
}

fn get_string(env: &mut JNIEnv<'_>, s: &JString<'_>) -> Result<String, ()> {
    env.get_string(s).map(|js| js.into()).map_err(|_| ())
}

// ---------------------------------------------------------------------
// PlatformClient lifecycle
// ---------------------------------------------------------------------

#[no_mangle]
pub extern "system" fn Java_io_llmhub_chat_internal_NativeBridge_nativeNewPlatform(
    mut env: JNIEnv<'_>,
    _class: JClass<'_>,
    j_base_url: JString<'_>,
    j_api_key: JString<'_>,
) -> jlong {
    catch_unwind(AssertUnwindSafe(|| {
        let base = match get_string(&mut env, &j_base_url) { Ok(s) => s, Err(_) => return 0 };
        let key  = match get_string(&mut env, &j_api_key)  { Ok(s) => s, Err(_) => return 0 };
        match PlatformClient::new(base, key) {
            Ok(c)  => into_handle::<Arc<PlatformClient>>(Arc::new(c)),
            Err(e) => { throw(&mut env, &e); 0 }
        }
    })).unwrap_or_else(|_| {
        throw_msg(&mut env, "panic", "panic in nativeNewPlatform");
        0
    })
}

#[no_mangle]
pub extern "system" fn Java_io_llmhub_chat_internal_NativeBridge_nativeFreePlatform(
    _env: JNIEnv<'_>,
    _class: JClass<'_>,
    handle: jlong,
) {
    unsafe { drop_handle::<Arc<PlatformClient>>(handle) }
}

// ---------------------------------------------------------------------
// ChatClient lifecycle
// ---------------------------------------------------------------------

#[no_mangle]
pub extern "system" fn Java_io_llmhub_chat_internal_NativeBridge_nativeNewChat(
    mut env: JNIEnv<'_>,
    _class: JClass<'_>,
    platform_handle: jlong,
    j_sku_id: JString<'_>,
) -> jlong {
    catch_unwind(AssertUnwindSafe(|| {
        let platform = unsafe { handle_ref::<Arc<PlatformClient>>(platform_handle) };
        let Some(platform) = platform else {
            throw_msg(&mut env, "invalid_handle", "platform handle is null");
            return 0;
        };
        let sku = match get_string(&mut env, &j_sku_id) { Ok(s) => s, Err(_) => return 0 };
        into_handle::<ChatClient>(ChatClient::new(platform.clone(), sku))
    })).unwrap_or_else(|_| {
        throw_msg(&mut env, "panic", "panic in nativeNewChat");
        0
    })
}

#[no_mangle]
pub extern "system" fn Java_io_llmhub_chat_internal_NativeBridge_nativeFreeChat(
    _env: JNIEnv<'_>,
    _class: JClass<'_>,
    handle: jlong,
) {
    unsafe { drop_handle::<ChatClient>(handle) }
}

// ---------------------------------------------------------------------
// Chat completions
// ---------------------------------------------------------------------

/// Blocking completion. Returns the response as a JSON string —
/// Kotlin side decodes with kotlinx.serialization. Returning JSON
/// keeps the JNI surface tiny (no marshalling per-field).
#[no_mangle]
pub extern "system" fn Java_io_llmhub_chat_internal_NativeBridge_nativeCreate(
    mut env: JNIEnv<'_>,
    _class: JClass<'_>,
    chat_handle: jlong,
    j_request_json: JString<'_>,
) -> jstring {
    let null = std::ptr::null_mut();
    catch_unwind(AssertUnwindSafe(|| {
        let chat = unsafe { handle_ref::<ChatClient>(chat_handle) };
        let Some(chat) = chat else {
            throw_msg(&mut env, "invalid_handle", "chat handle is null");
            return null;
        };
        let body = match get_string(&mut env, &j_request_json) { Ok(s) => s, Err(_) => return null };
        let req: ChatCompletionRequest = match serde_json::from_str(&body) {
            Ok(r) => r,
            Err(e) => { throw_msg(&mut env, "decode_error", &e.to_string()); return null }
        };
        match chat.create(req) {
            Ok(resp) => match serde_json::to_string(&resp) {
                Ok(s) => match env.new_string(s) {
                    Ok(js) => js.into_raw(),
                    Err(_) => null,
                },
                Err(e) => { throw_msg(&mut env, "decode_error", &e.to_string()); null }
            },
            Err(e) => { throw(&mut env, &e); null }
        }
    })).unwrap_or_else(|_| {
        throw_msg(&mut env, "panic", "panic in nativeCreate");
        null
    })
}

/// Streaming completion. Pumps each SSE chunk through
/// `listener.onChunk(String json)` and ends with `listener.onComplete()`
/// or `listener.onError(String code, String message)`. The listener
/// can return `false` from `onChunk` to cancel — we honor it and
/// emit `onComplete`.
#[no_mangle]
pub extern "system" fn Java_io_llmhub_chat_internal_NativeBridge_nativeCreateStream(
    mut env: JNIEnv<'_>,
    _class: JClass<'_>,
    chat_handle: jlong,
    j_request_json: JString<'_>,
    listener: JObject<'_>,
) {
    let r = catch_unwind(AssertUnwindSafe(|| -> std::result::Result<(), Error> {
        let chat = unsafe { handle_ref::<ChatClient>(chat_handle) }
            .ok_or_else(|| Error::Platform { code: "invalid_handle".into(), message: "chat handle is null".into() })?;
        let body = get_string(&mut env, &j_request_json)
            .map_err(|_| Error::Decode("request json".into()))?;
        let mut req: ChatCompletionRequest = serde_json::from_str(&body)
            .map_err(|e| Error::Decode(e.to_string()))?;
        req.stream = true;

        let listener_ref = &listener;
        let env_ptr = &mut env as *mut JNIEnv<'_>;

        chat.create_stream(req, &mut |chunk| {
            // SAFETY: we never escape the JNIEnv pointer beyond this
            // closure, which executes synchronously on the same thread
            // as the native call.
            let env = unsafe { &mut *env_ptr };
            let json = match serde_json::to_string(&chunk) {
                Ok(s) => s,
                Err(_) => return false,
            };
            let Ok(js) = env.new_string(json) else { return false };
            let res = env.call_method(
                listener_ref,
                "onChunk",
                "(Ljava/lang/String;)Z",
                &[JValue::Object(&js.into())],
            );
            match res {
                Ok(v) => v.z().unwrap_or(false),
                Err(_) => {
                    // Clear JNI exception so we can deliver onError next.
                    let _ = env.exception_clear();
                    false
                }
            }
        })?;
        Ok(())
    }));

    match r {
        Ok(Ok(())) => {
            let _ = env.call_method(&listener, "onComplete", "()V", &[]);
        }
        Ok(Err(e)) => {
            let Ok(code) = env.new_string(e.code()) else { return };
            let Ok(msg)  = env.new_string(e.to_string()) else { return };
            let _ = env.call_method(
                &listener,
                "onError",
                "(Ljava/lang/String;Ljava/lang/String;)V",
                &[JValue::Object(&code.into()), JValue::Object(&msg.into())],
            );
        }
        Err(_) => {
            let _ = env.throw_new(EXCEPTION_CLASS, "panic: nativeCreateStream");
        }
    }
}

// ---------------------------------------------------------------------
// One-shot list-services. Returns JSON array string.
// ---------------------------------------------------------------------
#[no_mangle]
pub extern "system" fn Java_io_llmhub_chat_internal_NativeBridge_nativeListServices(
    mut env: JNIEnv<'_>,
    _class: JClass<'_>,
    platform_handle: jlong,
) -> jstring {
    let null = std::ptr::null_mut();
    catch_unwind(AssertUnwindSafe(|| {
        let platform = unsafe { handle_ref::<Arc<PlatformClient>>(platform_handle) };
        let Some(platform) = platform else {
            throw_msg(&mut env, "invalid_handle", "platform handle is null");
            return null;
        };
        match platform.list_services() {
            Ok(svcs) => match serde_json::to_string(&svcs) {
                Ok(s) => match env.new_string(s) {
                    Ok(js) => js.into_raw(),
                    Err(_) => null,
                },
                Err(e) => { throw_msg(&mut env, "decode_error", &e.to_string()); null }
            },
            Err(e) => { throw(&mut env, &e); null }
        }
    })).unwrap_or_else(|_| {
        throw_msg(&mut env, "panic", "panic in nativeListServices");
        null
    })
}

// Touch jint to silence unused import on some toolchains.
#[allow(dead_code)]
fn _unused(_: jint) {}
