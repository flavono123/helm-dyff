# helm-dyff

![GitHub License](https://img.shields.io/github/license/flavono123/helm-dyff)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/flavono123/helm-dyff)
[![Go Report Card](https://goreportcard.com/badge/github.com/flavono123/helm-dyff)](https://goreportcard.com/report/github.com/flavono123/helm-dyff)

A helm3 plugin, dyff between the current release and new one with a new chart version and values.

## Concept

To dry-run the upgrade of a release with a new chart version, usually do this with [`dyff`](https://github.com/homeport/dyff), my favorite diff tool and too long and several times of `helm` calls:

```sh
❯ dyff bw ib \
  # from this, the current release's mainfests
  <(heml get manifest myrelease)  \
  # to this, the new one i want to upgrade
  <(helm template myrelease myrepo/mychart --version x.y.z -f <(helm get values myrelease))
```

Sometimes, the new values are required:

```sh
❯ dyff bw ib \
  <(heml get manifest myrelease)  \
  <(helm template myrelease myrepo/mychart --version x.y.z -f <(helm get values myrelease) -f new-values.yaml ...)
```

What if the curreent context is not in the same namespace as the release:

```sh
❯ dyff bw ib \
  <(heml get manifest myrelease -n mynamespace)  \
  <(helm template myrelease myrepo/mychart --version x.y.z -f <(helm get values myrelease -n mynamespace) -f new-values.yaml ... -n mynamespace)
```

## Use cases and examples

### New chart version

dyff with the new chart version:

```sh
❯ helm dyff upgrade myrelease myrepo/mychart -v x.y.z
```

### New values

dyff with the new values  *ammended*(would overwrite to the current release's one):

```sh
❯ helm dyff upgrade myrelease myrepo/mychart -f new-values.yaml
```

or just with the new values, disabling the default option:

```sh
❯ helm dyff upgrade myrelease myrepo/mychart -f new-values.yaml --ammend false
```

## Installation

```sh
❯ helm plugin install https://github.com/flavono123/helm-dyff.git
```

## TODO

- [ ] support `helm plugin update`
- [ ] unit tests
