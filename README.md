# Tape is for packaging applications

## Disclaimer

This project is an archived experiment that was done as part of Docker Labs and is no longer worked on.
It's been made available by Docker Labs team under the Apache license as it's deemed of potential interest to the community, however, it's no longer in active development.

## What is Tape?

Tape is a tool that can package an entire application as a self-contained (taped) OCI image that can be deployed to a
Kubernetes cluster. A taped OCI image contains all application components and Kubernetes resources required to run all
of the components together.

## What problem does it solve?

The process of building and deploying an application to Kubernetes is highly fragmented. There are many choices that
one has to make when implementing CI/CD. Leaving aside what CI vendor you pick, the starting point is the overall design of
a pipeline, repository structure, Dockerfiles, use of OCI registry, mapping of revisions to image tags, rules of deployment
based on tags, promotion between environments, as well as various kinds of ways to manage Kubernetes configs and how these
configs get deployed to any particular cluster.

Of course, there is some essential complexity, all CI pipelines will be different and everyone cannot use one and the same
tool for philosophical reasons. However, as most CI systems rely on shell scripts, it's quite challenging to define
contracts. What Tape does is all about having a simple artifact, which is a concrete contractual term, and Tape ensures
it has certain properties, for example, it references all formal dependencies by digest, i.e. dependencies that are OCI images
referenced from a canonical `image` field in a Kubernetes resource (e.g. `spec.containers[].image`).

Another issue that arises due to fragmentation is about collaboration between organisations that have made different choices,
despite all using OCI images and Kubernetes APIs.  To illustrate this, let's ask ourselves: "Why should there be any difference
in the mechanics of how my application is deployed to a Kubernetes cluster vs. someone else's application being deployed to the
same cluster?". Of course, it all comes down to Kubernetes API resources, but there is often a lot that happens in between.
Sometimes Helm is used, sometimes it's Kustomize, sometimes plain manifests are used with some bespoke automation around configs Git,
and there many examples of configs defined in scripting languages as well. Of course, it's very often a mix of a few approaches.
Even without anything too complicated, there is no such thing as a typical setup with Helm or Kustomize. The problem is that
the knowledge of what the choices are exactly and how some well-known tools might be used is not trivial or automatically
transferable, and there is little to say about the benefits of having something special.

Tape addresses the complexity by using OCI for the distribution of runnable code as well configuration required to run it
correctly on Kubernetes. Tape introduces a notion of application artifact without introducing any particular model of how
components are composed into an artifact. This model aims to be a layer of interoperability between different tools and
provide a logical supply chain entry point and location for storing metadata.

The best analogy is flatpack furniture. Presently, deployment of an application is as if flatpack hasn't been invented, so
when someone orders a wooden cabinet, all that arrives in a box is just the pieces of wood, they have to shop for nuts,
bolts, and tools. Of course, that might be desirable for some, as they have a well-stocked workshop with the best tools and
a decent selection of nuts and bolts. But did the box even include assembly instructions with the list of nuts and bolts
one has to buy?
That model doesn't scale to the consumer market. Of course, some consumers might have a toolbox, but very few will be able
to find the right nuts and bolts or even bother looking for any, they might just send the whole box back instead.
A taped image is like a flatpack package, it has everything needed as well as assembly instructions, without introducing
new complexity elsewhere and allows users to keep using their favourite tools.

To summarise, Tape reduces the complexity of application delivery by packaging the entire application as an OCI image, providing
a transferable artifact that includes config and all of the components, analogous to flatpack furniture. This notion of artifact
is very important because it helps to define a concise contract.
Tape also produces attestations about the provenance of the configuration source as well as any transforms it applies to the
source. The attestations are attached to the resulting OCI image, so it helps with security and observability as well.

## How does Tape work?

Tape can parse a directory with Kubernetes configuration and find all canonical references to application images.
If an image reference contains a digest, Tape will use it, otherwise it resolves it by making a registry API call.
For each of the images, Tape searches of all well-known related tags, such as external signatures, attestations and
SBOMs. Tape will make a copy of every application image and any tags related to it to a registry the user has specified.
Once images are copied, it updates manifests with new references and bundles the result in an OCI artifact pushed to
the same repo in the registry.

Copying of all application images and referencing by digest is performed to ensure the application and its configuration
are tightly coupled together to provide a single link in the supply chain as well as a single point of distribution
and access control for the whole application.

Tape also checks the VCS provenance of manifests, so if any manifest files are checked in Git, Tape will attest to what
Git repository each file came from, all of the revision metadata, and whether it's been modified or not.
Additionally, Tape attests to all key steps that it performs, e.g. original image references it detects and manifest
checksums. It stores the attestations using in-toto format in an OCI artifact.

## Usage

Tape has the following commands:

- `tape images` - examine images referenced by a given set of manifests before packaging them
- `tape package` - package an artifact and push it to a registry
- `tape pull` – download and extract contents and attestations from an existing artifact
- `tape view` – inspect an existing artifact

### Example

First, clone the repo and build `tape` binary:

```console
git clone -q git@github.com:errordeveloper/tape.git ; cd ./labs-brown-tape
(cd ./tape ; go build)
```

Clone podinfo app repo:
```console
(git clone -q https://github.com/stefanprodan/podinfo ; cd podinfo ; git switch --detach 6.4.1)
```

Examine podinfo manifests:
```console
$ ./tape/tape images --output-format text --manifest-dir ./podinfo/kustomize
INFO[0000] resolving image digests
INFO[0000] resolving related images
ghcr.io/stefanprodan/podinfo:6.4.1@sha256:92d43edf253c30782a1a9ceb970a718e6cb0454cff32a473e4f8a62dac355559
  Sources:
    ghcr.io/stefanprodan/podinfo:6.4.1 deployment.yaml:26:16@sha256:bb42d5f170c5c516b7c0f01ce16e82fff7b747c515e5a72dffe80395b52ac778
  Digest provided: false
  OCI manifests:
    sha256:4163972f9a84fde6c8db0e7d29774fd988a7668fe26c67ac09a90a61a889c92d  application/vnd.oci.image.manifest.v1+json  linux/amd64  1625
    sha256:e2d08f844f9af861a6ea5f47ce0f3fc45cfe3cc9f46f41ddbf8667f302711aea  application/vnd.oci.image.manifest.v1+json  linux/arm/v7  1625
    sha256:1eb30e81513b6cd96e51b4298ab49b8812c0c33403fc1b730dbf23c280af4cf7  application/vnd.oci.image.manifest.v1+json  linux/arm64  1625
    sha256:fd6487d2b151367fbb2b35576f5ac4bcf17d846f13133bf8f5f416eb796d2710  application/vnd.oci.image.manifest.v1+json  unknown/unknown  840
    sha256:ddb4ee5ac923648fc01af3610c9090f2f22bb66a2d3a600b82fe4cb09d15c39b  application/vnd.oci.image.manifest.v1+json  unknown/unknown  840
    sha256:d00c5c99beb6afddfcc3a6f3184bb91d14fdf27a41994542238751124f70332b  application/vnd.oci.image.manifest.v1+json  unknown/unknown  840
  Inline attestations: 3
  External attestations: 0
  Inline SBOMs: 3
  External SBOMs: 0
  Inline signatures: 0
  External signatures: 1
$
```

Package podinfo:
```console
$ ./tape/tape package --manifest-dir ./podinfo/kustomize --output-image ttl.sh/docker-labs-brown-tape/podinfo
INFO[0000] VCS info for "./podinfo/kustomize": {"unmodified":true,"path":"kustomize","uri":"https://github.com/stefanprodan/podinfo","isDir":true,"git":{"objectHash":"e5f73cd48e13a37c7f7c7b116d7da41e9adf7fd6","remotes":{"origin":["https://github.com/stefanprodan/podinfo"]},"reference":{"name":"HEAD","hash":"4892983fd12e3ffffcd5a189b1549f2ef26b81c2","type":"hash-reference"}}}
INFO[0000] resolving image digests
INFO[0000] resolving related images
INFO[0007] copying images
INFO[0012] copied images: ttl.sh/docker-labs-brown-tape/podinfo:app.98767129386790b1a06737587330605eed510345e9b40824f8d48813513a086a@sha256:92d43edf253c30782a1a9ceb970a718e6cb0454cff32a473e4f8a62dac355559, ttl.sh/docker-labs-brown-tape/podinfo:sha256-92d43edf253c30782a1a9ceb970a718e6cb0454cff32a473e4f8a62dac355559.sig@sha256:ed4e1649736c14982b5fe8a25c31a644ee99b7cec232d987c78fe1ab77000dce
INFO[0012] updating manifest files
INFO[0019] created package "ttl.sh/docker-labs-brown-tape/podinfo:config.ea816abb3c83c66181ff027115a84d930ec055ade76e3b7a861046df000bf75c@sha256:c4ef95c63f4572fbbdcc15270c2e2441b5aba753bc7d3a0cf8f7e3d8171b7c6d"
$
```

Store image name and config tag+digest as environment variables:
```console
podinfo_image=ttl.sh/docker-labs-brown-tape/podinfo
podinfo_config=${podinfo_image}:config.ea816abb3c83c66181ff027115a84d930ec055ade76e3b7a861046df000bf75c@sha256:c4ef95c63f4572fbbdcc15270c2e2441b5aba753bc7d3a0cf8f7e3d8171b7c6d
```

Examine the OCI index of the config image that's been created:
```console
$ crane manifest "${podinfo_config}" | jq .
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 625,
      "digest": "sha256:1f8b36b04801367cf9302ebadb7ff8a55d4a6b388007ccdc1b423657486952e2",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "artifactType": "application/vnd.docker.tape.content.v1alpha1.tar+gzip"
    },
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 1440,
      "digest": "sha256:6b8bd7bdb30a489db183930676c48f191b52f94be23c05ce035f8a3a8d330a53",
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "artifactType": "application/vnd.docker.tape.attest.v1alpha1.jsonl+gzip"
    }
  ],
  "annotations": {
    "org.opencontainers.image.created": "2023-08-30T11:05:44+01:00"
  }
}
$
```
Examine each of the two 2nd-level OCI manifests, the first one is for config contents, and the second for attestations:
```console
$ crane manifest "${podinfo_image}@$(crane manifest "${podinfo_config}" | jq -r '.manifests[0].digest')" | jq .
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.docker.tape.content.v1alpha1.tar+gzip",
    "size": 233,
    "digest": "sha256:3a5b16a8c592ad85f9b16563f47d03e5b66430e4db3f4260f18325e44e91942e"
  },
  "layers": [
    {
      "mediaType": "application/vnd.docker.tape.content.v1alpha1.tar+gzip",
      "size": 1182,
      "digest": "sha256:ea816abb3c83c66181ff027115a84d930ec055ade76e3b7a861046df000bf75c"
    }
  ],
  "annotations": {
    "application/vnd.docker.tape.content-interpreter.v1alpha1": "application/vnd.docker.tape.kubectl-apply.v1alpha1.tar+gzip",
    "org.opencontainers.image.created": "2023-08-30T11:05:44+01:00"
  }
}
$ crane manifest "${podinfo_image}@$(crane manifest "${podinfo_config}" | jq -r '.manifests[1].digest')" | jq .
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.docker.tape.attest.v1alpha1.jsonl+gzip",
    "size": 233,
    "digest": "sha256:3c162e42a2bbd7ff794312811a1da7a2a39289d4f41c7ac0f63c487a0eb3ae1a"
  },
  "layers": [
    {
      "mediaType": "application/vnd.docker.tape.attest.v1alpha1.jsonl+gzip",
      "size": 892,
      "digest": "sha256:9b4fdb608f604536f3740b4cdf9524f7d65ddd9e79c0d591383e2cc4970f4302"
    }
  ],
  "annotations": {
    "application/vnd.docker.tape.attestations-summary.v1alpha1": "eyJudW1TdGFtZW50ZXMiOjMsInByZWRpY2F0ZVR5cGVzIjpbImRvY2tlci5jb20vdGFwZS9NYW5pZmVzdERpci92MC4xIiwiZG9ja2VyLmNvbS90YXBlL09yaWdpbmFsSW1hZ2VSZWYvdjAuMSIsImRvY2tlci5jb20vdGFwZS9SZXNvbHZlZEltYWdlUmVmL3YwLjEiXSwic3ViamVjdCI6W3sibmFtZSI6Imt1c3RvbWl6ZS9kZXBsb3ltZW50LnlhbWwiLCJkaWdlc3QiOnsic2hhMjU2IjoiYmI0MmQ1ZjE3MGM1YzUxNmI3YzBmMDFjZTE2ZTgyZmZmN2I3NDdjNTE1ZTVhNzJkZmZlODAzOTViNTJhYzc3OCJ9fSx7Im5hbWUiOiJrdXN0b21pemUvaHBhLnlhbWwiLCJkaWdlc3QiOnsic2hhMjU2IjoiZDRiMmZmNmFmNjA3N2QwNjA2NTJiOTg0OWQwY2RhYjFlMjgxOGM2NGU1YjUwMWQ0MTRkODhjMjRkNWFiZGVmOCJ9fSx7Im5hbWUiOiJrdXN0b21pemUva3VzdG9taXphdGlvbi55YW1sIiwiZGlnZXN0Ijp7InNoYTI1NiI6Ijg5M2Y4OTYwZGVlZDM5NTkyZmQ0ZjQwMDRiNzBlMGIxYjZjNjkxYjRlNjI3MmRhMThhNDFjMTczNjBiNzcxZjUifX0seyJuYW1lIjoia3VzdG9taXplL3NlcnZpY2UueWFtbCIsImRpZ2VzdCI6eyJzaGEyNTYiOiJmMTg3NTY2ZjIxMmZjMTRlOWJlNjNkYWI3OWQ5ZGY1Y2ZhNzFkYzI4NDUwOWYyMjdlOWE0MjVkMTUyZmVlYzg1In19XX0K",
    "org.opencontainers.image.created": "2023-08-30T11:05:44+01:00"
  }
}
$
```

Store digests as variables:
```console
tape_config_digest="$(crane manifest "${podinfo_image}@$(crane manifest "${podinfo_config}" | jq -r '.manifests[0].digest')" | jq -r '.layers[0].digest')"
tape_attest_digest="$(crane manifest "${podinfo_image}@$(crane manifest "${podinfo_config}" | jq -r '.manifests[1].digest')" | jq -r '.layers[0].digest')"
```

Examine config contents:
```console
$ crane blob ${podinfo_image}@${tape_config_digest} | tar t
.
deployment.yaml
hpa.yaml
kustomization.yaml
service.yaml
$
```

Examine attestations:
```console
$ crane blob ${podinfo_image}@${tape_attest_digest} | gunzip | jq .
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "docker.com/tape/ManifestDir/v0.1",
  "subject": [
    {
      "name": "kustomize/deployment.yaml",
      "digest": {
        "sha256": "bb42d5f170c5c516b7c0f01ce16e82fff7b747c515e5a72dffe80395b52ac778"
      }
    },
    {
      "name": "kustomize/hpa.yaml",
      "digest": {
        "sha256": "d4b2ff6af6077d060652b9849d0cdab1e2818c64e5b501d414d88c24d5abdef8"
      }
    },
    {
      "name": "kustomize/kustomization.yaml",
      "digest": {
        "sha256": "893f8960deed39592fd4f4004b70e0b1b6c691b4e6272da18a41c17360b771f5"
      }
    },
    {
      "name": "kustomize/service.yaml",
      "digest": {
        "sha256": "f187566f212fc14e9be63dab79d9df5cfa71dc284509f227e9a425d152feec85"
      }
    }
  ],
  "predicate": {
    "containedInDirectory": {
      "path": "kustomize",
      "vcsEntries": {
        "providers": [
          "git"
        ],
        "entryGroups": [
          [
            {
              "unmodified": true,
              "path": "kustomize",
              "uri": "https://github.com/stefanprodan/podinfo",
              "isDir": true,
              "git": {
                "objectHash": "e5f73cd48e13a37c7f7c7b116d7da41e9adf7fd6",
                "remotes": {
                  "origin": [
                    "https://github.com/stefanprodan/podinfo"
                  ]
                },
                "reference": {
                  "name": "HEAD",
                  "hash": "4892983fd12e3ffffcd5a189b1549f2ef26b81c2",
                  "type": "hash-reference"
                }
              }
            },
            {
              "unmodified": true,
              "path": "kustomize/deployment.yaml",
              "uri": "https://github.com/stefanprodan/podinfo",
              "digest": {
                "sha256": "bb42d5f170c5c516b7c0f01ce16e82fff7b747c515e5a72dffe80395b52ac778"
              },
              "git": {
                "objectHash": "97c65ceffd80290eeab72dd9b7f94bdf59df9960",
                "remotes": {
                  "origin": [
                    "https://github.com/stefanprodan/podinfo"
                  ]
                },
                "reference": {
                  "name": "HEAD",
                  "hash": "4892983fd12e3ffffcd5a189b1549f2ef26b81c2",
                  "type": "hash-reference"
                }
              }
            },
            {
              "unmodified": true,
              "path": "kustomize/hpa.yaml",
              "uri": "https://github.com/stefanprodan/podinfo",
              "digest": {
                "sha256": "d4b2ff6af6077d060652b9849d0cdab1e2818c64e5b501d414d88c24d5abdef8"
              },
              "git": {
                "objectHash": "263e9128848695fec5ab76c7f864b11ec98c2149",
                "remotes": {
                  "origin": [
                    "https://github.com/stefanprodan/podinfo"
                  ]
                },
                "reference": {
                  "name": "HEAD",
                  "hash": "4892983fd12e3ffffcd5a189b1549f2ef26b81c2",
                  "type": "hash-reference"
                }
              }
            },
            {
              "unmodified": true,
              "path": "kustomize/kustomization.yaml",
              "uri": "https://github.com/stefanprodan/podinfo",
              "digest": {
                "sha256": "893f8960deed39592fd4f4004b70e0b1b6c691b4e6272da18a41c17360b771f5"
              },
              "git": {
                "objectHash": "470e464dfb87f30f136fb0626b16eddf2f874843",
                "remotes": {
                  "origin": [
                    "https://github.com/stefanprodan/podinfo"
                  ]
                },
                "reference": {
                  "name": "HEAD",
                  "hash": "4892983fd12e3ffffcd5a189b1549f2ef26b81c2",
                  "type": "hash-reference"
                }
              }
            },
            {
              "unmodified": true,
              "path": "kustomize/service.yaml",
              "uri": "https://github.com/stefanprodan/podinfo",
              "digest": {
                "sha256": "f187566f212fc14e9be63dab79d9df5cfa71dc284509f227e9a425d152feec85"
              },
              "git": {
                "objectHash": "9450823d5a09afc116a37ee16da12f53a6f4836d",
                "remotes": {
                  "origin": [
                    "https://github.com/stefanprodan/podinfo"
                  ]
                },
                "reference": {
                  "name": "HEAD",
                  "hash": "4892983fd12e3ffffcd5a189b1549f2ef26b81c2",
                  "type": "hash-reference"
                }
              }
            }
          ]
        ]
      }
    }
  }
}
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "docker.com/tape/OriginalImageRef/v0.1",
  "subject": [
    {
      "name": "kustomize/deployment.yaml",
      "digest": {
        "sha256": "bb42d5f170c5c516b7c0f01ce16e82fff7b747c515e5a72dffe80395b52ac778"
      }
    }
  ],
  "predicate": {
    "foundImageReference": {
      "reference": "ghcr.io/stefanprodan/podinfo:6.4.1",
      "line": 26,
      "column": 16
    }
  }
}
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "docker.com/tape/ResolvedImageRef/v0.1",
  "subject": [
    {
      "name": "kustomize/deployment.yaml",
      "digest": {
        "sha256": "bb42d5f170c5c516b7c0f01ce16e82fff7b747c515e5a72dffe80395b52ac778"
      }
    }
  ],
  "predicate": {
    "resolvedImageReference": {
      "reference": "ghcr.io/stefanprodan/podinfo:6.4.1@sha256:92d43edf253c30782a1a9ceb970a718e6cb0454cff32a473e4f8a62dac355559",
      "line": 26,
      "column": 16,
      "alias": "podinfo"
    }
  }
}
$
```

## FAQ

### What configuration formats does Tape support, does it support any kind of templating?

Tape supports plain JSON and YAML manifest, which was the scope of the original experiment.
If the project was to continue, it could accommodate a variety of popular templating options,
e.g. CUE, Helm, and scripting languages, paving a way for a universal artifact format.

### How does Tape relate to existing tools?

Many existing tools in this space help with some aspects of handling Kubernetes resources. These tools operate on
either loosely coupled collections of resouces (like Kustomize), or opinionated application package formats (most
notably Helm). One of the goals of Tape is to abstract the use of any tools that already exist while paving the way
for innovation. Tape will attempt to integrate with most of the popular tools, and enable anyone to deploy applications
from taped images without having to know if under the hood it will use Kustomize, Helm, just plain manifest, or something
else entirely. The other goal is that users won't need to know about Tape either, perhaps someday `kubectl apply` could
support OCI artifacts and there could be different ways of building the artifacts.

### What kind of applications can Tape package?

Tape doesn't infer an opinion of how the application is structured, or what it consists of or doesn't consist of. It doesn't
present any application definition format, it operates on plain Kubernetes manifests found in a directory.

### Does Tape provide SBOMs?

Tape doesn't explicitly generate or process SBOMs, but fundamentally it could provide functionality around that.

## Acknowledgments & Prior Art

What Tape does is very much in the spirit of Docker images, but it extends the idea by shifting the perspective to configuration
as an entry point to a map of dependencies, as opposed to the forced separation of app images and configuration.

It's not a novelty to package configuration in OCI, there are many examples of this, yet that in itself doesn't provide for interoperability.
One could imagine something like Tape as a model that abstracts configuration tooling so that end-users don't need to think about whether
a particular app needs to be deployed with Helm, Kustomize, or something else.

Tape was directly inspired by [flux push artifact](https://fluxcd.io/flux/cheatsheets/oci-artifacts/). Incidentally, it also resembles
some of the aspects of CNAB, but it is much smaller in scope.
