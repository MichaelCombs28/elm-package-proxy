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

#### Standard Elm Publish

Just add `"private": true` to your elm.json and you're good to go.
Conflicting packages that were deployed to both the official and private repositories,
will default to using the private package.

#### Manual Upload

In the case of wanting to avoid all of the `elm publish` formalities, you can upload
a package as a zip file.

### Creating a kernel Package

There are a couple of ways to create a kernel package using `elm-proxy`.

#### Using official compiler

Only repositories existing within the `elm` or `elm-explorations` namespaces are allowed to
provide kernel code without modifications.

You can only use these by uploading the package manually.

#### Using forked compiler

Only a couple of code changes are required to allow kernel code in a user provided package.
Unfortunately, a lot of rewiring would be required to allow kernel code on an application
level, so hopefully native in package form is good enough for your usecase.

## Roadmap
