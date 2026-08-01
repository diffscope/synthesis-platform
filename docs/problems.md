# HTTP Problem Details

DiffScope Synthesis Platform reports HTTP errors using the
[`application/problem+json`](https://www.rfc-editor.org/rfc/rfc9457.html)
format defined by RFC 9457.

Every standalone problem response contains these members:

| Member | Description |
| --- | --- |
| `state` | Always `ERROR`. This preserves the state used by synthesis responses and events. |
| `type` | A stable URI reference identifying the problem type. |
| `title` | A short, stable English summary of the problem type. |
| `status` | The HTTP status code returned with the response. |
| `detail` | A human-readable explanation of this occurrence. |

Problem responses do not contain an `instance` member.

When a streaming request has already started an `application/x-ndjson`
response, a later error is emitted as a problem-shaped stream event. Such an
event contains `state`, `type`, `title`, and `detail`, together with any
problem-specific members described below. It omits `status` because the HTTP
status has already been committed as `200`.

## Problem types

| Type | Title | Status | Additional members |
| --- | --- | ---: | --- |
| `/problems/internal_error` | `Internal error` | 500 | None. |
| `/problems/unknown_arch` | `Unknown arch` | 404 | `arch` contains the unrecognized architecture ID. |
| `/problems/singer_not_exist` | `Singer not exist` | 404 | `arch` contains the current architecture ID and `singer` contains the missing singer ID. |
| `/problems/singer_config_invalid` | `Singer config invalid` | 422 | `errors` describes one or more configuration validation failures. |
| `/problems/invalid_parameter` | `Invalid parameter` | 422 | `parameter` identifies the parameter and the kind of parameter error. |
| `/problems/singers_unmixable` | `Singers unmixable` | 422 | `arch` contains the current architecture ID and `singers` contains the two incompatible singer IDs. |
| `/problems/validation_error` | `Validation error` | 400 | `errors` describes one or more request-body validation failures. |

## Validation errors

The `errors` member is an array. Each item has the following members:

| Member | Description |
| --- | --- |
| `pointer` | A JSON Pointer fragment locating the relevant request value or its nearest enclosing request area. `#` identifies the complete request body. |
| `type` | A stable, machine-readable validation error type, such as `required`, `invalid_type`, `invalid_json`, `invalid_mix`, `invalid_value`, `invalid_singer_extra`, `unknown_speaker`, or `configuration_invalid`. |
| `detail` | A human-readable explanation of that validation failure. |

For `/problems/validation_error`, pointers locate values in the request body.
For `/problems/singer_config_invalid`, an architecture can report a pointer to
the nearest request area when the exact failure originates in singer package
configuration rather than in a single JSON member.

Example:

```json
{
  "state": "ERROR",
  "type": "/problems/validation_error",
  "title": "Validation error",
  "status": 400,
  "detail": "The request body is invalid.",
  "errors": [
    {
      "pointer": "#/context/arch",
      "type": "required",
      "detail": "is required"
    }
  ]
}
```

## Invalid parameters

The `parameter` member has this shape:

| Member | Description |
| --- | --- |
| `id` | The architecture parameter ID that caused the error. |
| `error_type` | The kind of parameter error. |

Current `error_type` values are:

| Value | Meaning |
| --- | --- |
| `missing` | A required parameter was not supplied. |
| `retake_required` | The requested operation requires retake information for the parameter. |
| `retake_not_supported` | The parameter does not support retake. |
| `invalid_value` | A parameter value or one of its properties is invalid for the architecture. |

Example:

```json
{
  "state": "ERROR",
  "type": "/problems/invalid_parameter",
  "title": "Invalid parameter",
  "status": 422,
  "detail": "missing pitch parameter",
  "parameter": {
    "id": "pitch",
    "error_type": "missing"
  }
}
```

## Singer identity and mixing

A missing singer is reported with both its architecture and singer ID:

```json
{
  "state": "ERROR",
  "type": "/problems/singer_not_exist",
  "title": "Singer not exist",
  "status": 404,
  "detail": "",
  "arch": "diffsinger",
  "singer": "example-singer-id"
}
```

An incompatible mix reports the first pair found to be incompatible:

```json
{
  "state": "ERROR",
  "type": "/problems/singers_unmixable",
  "title": "Singers unmixable",
  "status": 422,
  "detail": "singers use different acoustic inference",
  "arch": "diffsinger",
  "singers": [
    "first-singer-id",
    "second-singer-id"
  ]
}
```
