# Elm Package Proxy

[[_TOC_]]

Proxy server and elm package cache manager enabling support for both
kernel & private packages.

## Status

Not Production Ready

## Installation

### Creating a Trusted Cert & Private Key

You must create a certificate pair to connect with `package.elm-lang.org` and
optionally `github.com` + `api.github.com` if you would like to use private packages
seamlessly.

### Docker

### Source

## Usage

TODO

### Docker

TODO

### Local

TODO

### CLI Arguments

TODO

### Creating a Private Package

Private packages can be created a couple of ways. The goal was to seamlessly integrate with
existing elm compiler for the sake of private packages alone.

#### Private Namespace

A private namespace allows the use of the standard `elm publish` mechanism,
as well as the manual submission a private package using the [elm-alchemy](https://github.com/MichaelCombs28/TBD)
tool without the need to configure github certs + API tokens.

When a private namespace is created, any package published to that namespace eg. `MichaelCombs28/elm-base85` will create that package and store it privately.

Using this package you would use:

```sh
elm install privateMichaelCombs28/elm-base85
```

Package names are prefixed with `private` in order to avoid conflicts with the official repositories.
Conflict resolution is on the roadmap since this mechanism does not cover all edge cases.

The main exception to this rule are namespaces prefixed with `elm/` or `elm-explorations/`.
These namespace prefixes indicate the user wants to run kernel code provided by the user using `elm-alchemy`
without modifications to the elm compiler.

#### Private GitHub repository

Currently, the elm compiler only supports searching github for packages,
so as part of the proxy, a GitHub handler has been provided that appends a user provided Personal API token
to the API call to retrieve the zipball, check the tag, etc.

### Creating a kernel Package

There are a couple of ways to create a kernel package using `elm-proxy`.

#### Using official compiler

Only repositories existing within the `elm` or `elm-explorations` namespaces are allowed to
provide kernel code without modifications.

You must create a private namespace named `elm` or `elm-explorations` and publish
packages to it using `elm-alchemy`.

#### Using forked compiler

Only a couple of code changes are required to allow kernel code in a user provided package.
Unfortunately, a lot of rewiring would be required to allow kernel code on an application
level, so hopefully native in package form is good enough for your usecase.

## Roadmap
