package auth

default allow = false

tokens := {
    "Bearer eZNG6I3VTU28Qe",
}

allow {
    tokens[input.token]
}
