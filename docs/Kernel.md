# Kernel

[[_TOC_]]

## Overview

Kernel packages are what used to be called "native" packages which use native
Javascript code to implement functionality which can be called upon by
elm functions as a sort of foreign function interface.

The issue with Kernel code is that it was not meant to be written by anyone outside
of either the [elm](https://github.com/elm/) or [elm-explorations](https://github.com/elm-explorations) organizations.

`elm-package-proxy` helps with this by providing the user with a private package repository, and a forked compiler
binary with a two line code change removing these limitations for both packages, as well as application type projects.

- [Elm Kernel Compiler](https://github.com/MichaelCombs28/elm-kernel-compiler)

## Nomenclature of an Elm Kernel Package

This is the layout of an example project called [elm-private-package](https://github.com/MichaelCombs28/elm-private-package):

```sh
.
├── elm.json
├── LICENSE
├── README.md
└── src
    ├── Elm
    │   └── Kernel
    │       └── Private.js
    └── Private.elm
```

The compiler will look the imports in `Private.elm` and if it sees an `Elm.Kernel` prefixed import, will
crawl that area looking for Kernel modules written in Javascript.

```elm
-- Import
import Elm.Kernel.Private

-- Calling on Kernel function
privateString : String -> String -> String
privateString a b =
    Elm.Kernel.Private.hash a b
```

`Elm.Kernel.Private` denotes that a file named `Private.js` exists on path `./src/Elm/Kernel`

**Note**: Kernel namespaces cannot be aliased with `as` syntax.

```javascript
/*

*/

// PRIVATE

var crypto = require("crypto");

var _Private_hash = F2(function (key, value) {
  var shasum = crypto.createHash("sha1");
  shasum.update(key + value);
  return shasum.digest("hex");
});
```

These Kernel files require a multi line comment at the top, which can be used
to import either known kernel, see [elm/core](https://github.com/elm/core) for an
example.

## Creating an Elm Kernel Package

## FAQs

### Common Errors
