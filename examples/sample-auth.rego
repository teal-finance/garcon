package auth

default allow = false

tokens := {
    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lc3BhY2UiOiJGcmVlUGxhbiIsInVzZXJuYW1lIjoiIiwiZXhwIjoxNjY4Njk1OTU2fQ.45Ku3S7ljXKtrbxwg_sAJam12RMHenC2GYlAa-nXcgo",
}

allow {
    tokens[input.token]
}
