workspace(
    name = "skia_infra",

    # Must be kept in sync with the npm_install rules invoked below.
    managed_directories = {
        "@npm": ["node_modules"],
    },
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("//bazel:gcs_mirror.bzl", "gcs_mirror_url")

# Read the instructions in //bazel/rbe/generated/README.md before updating this repository.
#
# We load bazel-toolchains here, rather than closer where it's first used (RBE container toolchain),
# because the grpc_deps() macro (invoked below) will pull an old version of bazel-toolchains if it's
# not already defined.
http_archive(
    name = "bazel_toolchains",
    sha256 = "179ec02f809e86abf56356d8898c8bd74069f1bd7c56044050c2cd3d79d0e024",
    strip_prefix = "bazel-toolchains-4.1.0",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz",
        "https://github.com/bazelbuild/bazel-toolchains/releases/download/4.1.0/bazel-toolchains-4.1.0.tar.gz",
    ],
)

#################
# Python rules. #
#################

http_archive(
    name = "rules_python",
    sha256 = "cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd",
    strip_prefix = "rules_python-0.8.1",
    urls = gcs_mirror_url(
        sha256 = "cdf6b84084aad8f10bf20b46b77cb48d83c319ebe6458a18e9d2cebf57807cdd",
        # Update after a release with https://github.com/bazelbuild/rules_python/pull/1032 lands
        url = "https://github.com/bazelbuild/rules_python/archive/refs/tags/0.8.1.tar.gz",
    ),
)

load("@rules_python//python:repositories.bzl", "python_register_toolchains")

# Hermetically downloads Python 3.
python_register_toolchains(
    name = "python3_10",
    # Taken from
    # https://github.com/bazelbuild/rules_python/blob/63805ab7a65b90c4723ecbe18f2c88da714e5d7a/python/versions.bzl#L94.
    python_version = "3.10",
)

##############################
# Go rules and dependencies. #
##############################

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "91585017debb61982f7054c9688857a2ad1fd823fc3f9cb05048b0025c47d023",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "d3fa66a39028e97d76f9e2db8f1b0c11c099e8e01bf363a923074784e451f809",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.33.0/bazel-gazelle-v0.33.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("//:go_repositories.bzl", "go_repositories")

# gazelle:repository_macro go_repositories.bzl%go_repositories
go_repositories()

go_rules_dependencies()

go_register_toolchains(version = "1.21.4")

gazelle_dependencies()

##########################
# Other Go dependencies. #
##########################

load("//bazel/external:go_googleapis_compatibility_hack.bzl", "go_googleapis_compatibility_hack")

# Compatibility hack to make the github.com/bazelbuild/remote-apis Go module work with rules_go
# v0.41.0 or newer. See the go_googleapis() rule's docstring for details.
go_googleapis_compatibility_hack(
    name = "go_googleapis",
)

# Needed by @com_github_bazelbuild_remote_apis.
http_archive(
    name = "com_google_protobuf",
    sha256 = "b8ab9bbdf0c6968cf20060794bc61e231fae82aaf69d6e3577c154181991f576",
    strip_prefix = "protobuf-3.18.1",
    urls = gcs_mirror_url(
        sha256 = "b8ab9bbdf0c6968cf20060794bc61e231fae82aaf69d6e3577c154181991f576",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v3.18.1/protobuf-all-3.18.1.tar.gz",
    ),
)

# Originally, we pulled protobuf dependencies as follows:
#
#     load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")
#     protobuf_deps()
#
# The protobuf_deps() macro brings in a bunch of dependencies, but by copying the macro body here
# and removing dependencies one by one, "rules_proto" was identified as the only dependency that is
# required to build this repository.
http_archive(
    name = "rules_proto",
    sha256 = "a4382f78723af788f0bc19fd4c8411f44ffe0a72723670a34692ffad56ada3ac",
    strip_prefix = "rules_proto-f7a30f6f80006b591fa7c437fe5a951eb10bcbcf",
    urls = ["https://github.com/bazelbuild/rules_proto/archive/f7a30f6f80006b591fa7c437fe5a951eb10bcbcf.zip"],
)

http_archive(
    name = "com_google_googleapis",
    sha256 = "38701e513aff81c89f0f727e925bf04ac4883913d03a60cdebb2c2a5f10beb40",
    strip_prefix = "googleapis-86fa44cc5ee2136e87c312f153113d4dd8e9c4de",
    urls = [
        "https://github.com/googleapis/googleapis/archive/86fa44cc5ee2136e87c312f153113d4dd8e9c4de.tar.gz",
    ],
)

# Needed by @com_github_bazelbuild_remote_apis for the googleapis protos.
http_archive(
    name = "googleapis",
    build_file = "BUILD.googleapis",
    sha256 = "7b6ea252f0b8fb5cd722f45feb83e115b689909bbb6a393a873b6cbad4ceae1d",
    strip_prefix = "googleapis-143084a2624b6591ee1f9d23e7f5241856642f4d",
    urls = gcs_mirror_url(
        sha256 = "7b6ea252f0b8fb5cd722f45feb83e115b689909bbb6a393a873b6cbad4ceae1d",
        # b/267219467
        url = "https://github.com/googleapis/googleapis/archive/143084a2624b6591ee1f9d23e7f5241856642f4d.zip",
    ),
)

load("@com_google_googleapis//:repository_rules.bzl", googleapis_imports_switched_rules_by_language = "switched_rules_by_language")

googleapis_imports_switched_rules_by_language(
    name = "com_google_googleapis_imports",
    go = True,
    grpc = True,
)

# Needed by @com_github_bazelbuild_remote_apis for gRPC.
http_archive(
    name = "com_github_grpc_grpc",
    sha256 = "b391a327429279f6f29b9ae7e5317cd80d5e9d49cc100e6d682221af73d984a6",
    strip_prefix = "grpc-93e8830070e9afcbaa992c75817009ee3f4b63a0",  # v1.24.3 with fixes
    urls = gcs_mirror_url(
        sha256 = "b391a327429279f6f29b9ae7e5317cd80d5e9d49cc100e6d682221af73d984a6",
        # Fix after https://github.com/grpc/grpc/issues/32259 is resolved
        url = "https://github.com/grpc/grpc/archive/93e8830070e9afcbaa992c75817009ee3f4b63a0.zip",
    ),
)

# Originally, we pulled gRPC dependencies as follows:
#
#     load("@com_github_grpc_grpc//bazel:grpc_deps.bzl", "grpc_deps")
#     grpc_deps()
#
# The grpc_deps() macro brings in a bunch of dependencies, but by copying the macro body here
# and removing dependencies one by one, "zlib" was identified as the only dependency that is
# required to build this repository.
http_archive(
    name = "zlib",
    build_file = "@com_github_grpc_grpc//third_party:zlib.BUILD",
    sha256 = "6d4d6640ca3121620995ee255945161821218752b551a1a180f4215f7d124d45",
    strip_prefix = "zlib-cacf7f1d4e3d44d871b605da3b647f07d718623f",
    url = "https://github.com/madler/zlib/archive/cacf7f1d4e3d44d871b605da3b647f07d718623f.tar.gz",
)

###################################################
# JavaScript / TypeScript rules and dependencies. #
###################################################

http_archive(
    name = "build_bazel_rules_nodejs",
    sha256 = "0fad45a9bda7dc1990c47b002fd64f55041ea751fafc00cd34efb96107675778",
    urls = gcs_mirror_url(
        sha256 = "0fad45a9bda7dc1990c47b002fd64f55041ea751fafc00cd34efb96107675778",
        url = "https://github.com/bazelbuild/rules_nodejs/releases/download/5.5.0/rules_nodejs-5.5.0.tar.gz",
    ),
)

load("@build_bazel_rules_nodejs//:repositories.bzl", "build_bazel_rules_nodejs_dependencies")

build_bazel_rules_nodejs_dependencies()

load("@build_bazel_rules_nodejs//:index.bzl", "node_repositories", "npm_install")

node_repositories(
    node_version = "16.12.0",
    # We don't use Yarn directly, but the Bazel rules in the rules_nodejs repository do.
    yarn_version = "1.22.11",
)

# The npm_install rule manages the node_modules directory, and runs anytime the package.json or
# package-lock.json file changes. It also extracts any Bazel rules distributed in an NPM package.
#
# There must be one npm_install rule for each package.json file in this repository. Any node_modules
# directories managed by npm_install rules must be mentioned in the workspace() rule at the top of
# this file.
npm_install(
    name = "npm",
    exports_directories_only = False,
    package_json = "//:package.json",
    package_lock_json = "//:package-lock.json",
    symlink_node_modules = True,
)

load(
    "@build_bazel_rules_nodejs//toolchains/esbuild:esbuild_repositories.bzl",
    "esbuild_repositories",
)

esbuild_repositories(npm_repository = "npm")

################################
# Sass rules and dependencies. #
################################

http_archive(
    name = "io_bazel_rules_sass",
    sha256 = "6cca1c3b77185ad0a421888b90679e345d7b6db7a8c9c905807fe4581ea6839a",
    strip_prefix = "rules_sass-1.49.8",
    urls = gcs_mirror_url(
        sha256 = "6cca1c3b77185ad0a421888b90679e345d7b6db7a8c9c905807fe4581ea6839a",
        # Fix after https://github.com/bazelbuild/rules_sass/issues/147 is addressed
        url = "https://github.com/bazelbuild/rules_sass/archive/1.49.8.zip",
    ),
)

load("@io_bazel_rules_sass//:defs.bzl", "sass_repositories")

sass_repositories()

##################################
# Docker rules and dependencies. #
##################################

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
    strip_prefix = "rules_docker-0.24.0",
    urls = gcs_mirror_url(
        sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
        url = "https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz",
    ),
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

# This is required by the toolchain_container rule.
load(
    "@io_bazel_rules_docker//repositories:go_repositories.bzl",
    container_go_deps = "go_deps",
)

container_go_deps()

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

# Provides the pkg_tar rule, needed by the skia_app_container macro.
#
# See https://github.com/bazelbuild/rules_pkg/tree/main/pkg.
http_archive(
    name = "rules_pkg",
    sha256 = "038f1caa773a7e35b3663865ffb003169c6a71dc995e39bf4815792f385d837d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
    ],
)

load("@rules_pkg//:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

##################
# Miscellaneous. #
##################

load("@bazel_toolchains//rules/exec_properties:exec_properties.bzl", "rbe_exec_properties")

# Defines a local repository named "exec_properties" which defines constants such as NETWORK_ON.
# These constants are used by the //:rbe_custom_platform build rule.
#
# See https://github.com/bazelbuild/bazel-toolchains/tree/master/rules/exec_properties.
rbe_exec_properties(
    name = "exec_properties",
)

######################
# Docker containers. #
######################

# This is a pinned version of JS fiddle - we use the canvaskit.js/canvaskit.wasm inside it
# when running apps (e.g. skottie.skia.org) locally. All our apps (except debugger) use the stock
# version of CanvasKit, so they can share this. If there is an update to CanvasKit APIs and we want
# to test them out locally, we should update this to a newer version. See the k8s-config repo
# for a recent commit to use.
container_pull(
    name = "pinned_jsfiddle",
    digest = "sha256:2d601a86398166b7d87b1cbe69005ac1b0302d22c72dca5a8d7d4340d79c33b8",
    registry = "gcr.io",
    repository = "skia-public/jsfiddle-final",
)

# Debugger's version of CanvasKit is built with different flags
container_pull(
    name = "pinned_debugger",
    digest = "sha256:d63e5ee97f80701a7bf5c0f69da9858a9ca9a349c3f07de39e7d261527c7021d",
    registry = "gcr.io",
    repository = "skia-public/debugger-app-final",
)

# This is an arbitrary version of the public Alpine image. Given our current rules, we must pull
# a docker container and extract some files, even if we are just building local versions (e.g.
# of debugger or skottie), so this is the image for that.
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    registry = "index.docker.io",
    repository = "alpine",
)

# Pulls the gcr.io/skia-public/basealpine container, needed by the skia_app_container macro.
container_pull(
    name = "basealpine",
    digest = "sha256:35a26930eb37b90cb0bdf69050e363bd749b56656963b78c8c4b4758a5aea8fa",
    registry = "gcr.io",
    repository = "skia-public/basealpine",
)

# Pulls the gcr.io/skia-public/base-cipd container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "base-cipd",
    digest = "sha256:d5c4239eac1efa961dd8d2cc8cb998a15aa4f4b59838b6c694839cfe032fbfe8",
    registry = "gcr.io",
    repository = "skia-public/base-cipd",
)

# Pulls the gcr.io/skia-public/cd-base container, needed by some apps that use the
# skia_app_container macro.
container_pull(
    name = "cd-base",
    digest = "sha256:17e18164238a4162ce2c30b7328a7e44fbe569e56cab212ada424dc7378c1f5f",
    registry = "gcr.io",
    repository = "skia-public/cd-base",
)

# Pulls the gcr.io/skia-public/skia-build-tools container, needed by some apps that
# build skia.
container_pull(
    name = "skia-build-tools",
    digest = "sha256:28cc48a073ac1f35f468c1b725e331b626791b35edb18696f30891c4f047d236",
    registry = "gcr.io",
    repository = "skia-public/skia-build-tools",
)

# Pulls the gcr.io/skia-public/docsyserver-base container, needed by docsyserver.
container_pull(
    name = "docsyserver-base",
    digest = "sha256:ca63ba5a92e1adbe49eb6e6e1262ee4724e572f87e54eea01737cbb2a73fde6c",
    registry = "gcr.io",
    repository = "skia-public/docsyserver-base",
)

# Pulls the envoyproxy/envoy-alpine:v1.16.1 container, needed by skfe.
container_pull(
    name = "envoy_alpine",
    digest = "sha256:061559f887b6b7980ea1ebb5af636079858d8b0f51cd803b9fe16f87811ff7d3",
    registry = "index.docker.io",
    repository = "envoyproxy/envoy-alpine",
)

# Pulls the node:17-alpine container, needed by jsdoc.
container_pull(
    name = "node_alpine",
    digest = "sha256:44b4db12ba2899f92786aa7e98782eb6430e81d92488c59144a567853185c2bb",
    registry = "index.docker.io",
    repository = "node",
)

# Pulls the https://gcr.io/cloud-builders/kubectl container, needed by apps that use kubectl.
container_pull(
    name = "kubectl",
    digest = "sha256:63553d791cbdd3aa9fc2bc0b3a6a6d33130c1b8927b2db368c756aa45c89a356",  # 25 Oct 2023
    registry = "gcr.io",
    repository = "cloud-builders/kubectl",
)

# Pulls the gcr.io/google.com/cloudsdktool/cloud-sdk:latest container needed by Perf backup.
container_pull(
    name = "cloudsdk",
    digest = "sha256:900b74f1fb2c9f93c6d4b121a7f23981143496f36aacb72e596ccaedad640cf1",  # @latest as of Apr 27, 2022.
    registry = "gcr.io",
    repository = "google.com/cloudsdktool/cloud-sdk",
)

##################
# CIPD packages. #
##################

load("//bazel/external:cipd_install.bzl", "all_cipd_files", "cipd_install")

cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
    tag = "version:2.29.2.chromium.6",
)

cipd_install(
    name = "git_amd64_windows",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/windows-amd64",
    postinstall_cmds_win = [
        "mkdir etc",
        "bin/git.exe config --system user.name \"Bazel Test User\"",
        "bin/git.exe config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/windows-amd64/+/version:2.29.2.chromium.6
    sha256 = "9caaf2c6066bdcfa94f917323c4031cf7e32572848f8621ecd0d328babee220a",
    tag = "version:2.29.2.chromium.6",
)

cipd_install(
    name = "vpython_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/tools/luci/vpython/linux-amd64",
    # From https://chrome-infra-packages.appspot.com/p/infra/tools/luci/vpython/linux-amd64/+/git_revision:7989c7a87b25083bd8872f9216ba4819c18ab097
    sha256 = "1de06f1727bde7ef9eaae901944adead46dd2b7ddda1e962fff29ee431b0e746",
    tag = "git_revision:7989c7a87b25083bd8872f9216ba4819c18ab097",
)

cipd_install(
    name = "cpython3_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/cpython3/linux-amd64",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/cpython3/linux-amd64/+/version:2@3.8.10.chromium.19
    sha256 = "4ba68650a271a80a565a619ed2419f4cf1344525b63798608ce3b8cef63a9244",
    tag = "version:2@3.8.10.chromium.19",
)

cipd_install(
    name = "cabe_replay_data",
    build_file_content = all_cipd_files(),
    cipd_package = "skia/bots/cabe",
    # From https://chrome-infra-packages.appspot.com/p/skia/bots/cabe/+/D3RiS_D3UDI2YYBNBOETOqdxD6c0xdY34SV2A6OKEIoC
    sha256 = "0f74624bf0f750323661804d04e1133aa7710fa734c5d637e1257603a38a108a",
    tag = "version:4",
)

#############################################################
# Google Cloud SDK (needed for the Google Cloud Emulators). #
#############################################################

load("//bazel/external:google_cloud_sdk.bzl", "google_cloud_sdk")

google_cloud_sdk(name = "google_cloud_sdk")

##################################################
# CockroachDB (used as an "emulator" for tests). #
##################################################

http_archive(
    name = "cockroachdb_linux",
    build_file_content = """
filegroup(
    name = "all_files",
    srcs = glob(["**/*"]),
    visibility = ["//visibility:public"]
)
""",
    # https://www.cockroachlabs.com/docs/v21.1/install-cockroachdb-linux does not currently
    # provide SHA256 signatures. kjlubick@ downloaded this file and computed this sha256 signature.
    sha256 = "05293e76dfb6443790117b6c6c05b1152038b49c83bd4345589e15ced8717be3",
    strip_prefix = "cockroach-v21.1.9.linux-amd64",
    urls = gcs_mirror_url(
        sha256 = "05293e76dfb6443790117b6c6c05b1152038b49c83bd4345589e15ced8717be3",
        url = "https://binaries.cockroachdb.com/cockroach-v21.1.9.linux-amd64.tgz",
    ),
)

#################################################################################
# Google Chrome and Fonts (needed for Karma and Puppeteer tests, respectively). #
#################################################################################

load("//bazel/external:google_chrome.bzl", "google_chrome")

google_chrome(name = "google_chrome")

##########################
# Buildifier (prebuilt). #
##########################

http_file(
    name = "buildifier_linux_amd64",
    downloaded_file_path = "buildifier",
    executable = True,
    sha256 = "52bf6b102cb4f88464e197caac06d69793fa2b05f5ad50a7e7bf6fbd656648a3",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "52bf6b102cb4f88464e197caac06d69793fa2b05f5ad50a7e7bf6fbd656648a3",
        url = "https://github.com/bazelbuild/buildtools/releases/download/5.1.0/buildifier-linux-amd64",
    ),
)

http_file(
    name = "buildifier_macos_arm64",
    downloaded_file_path = "buildifier",
    executable = True,
    sha256 = "745feb5ea96cb6ff39a76b2821c57591fd70b528325562486d47b5d08900e2e4",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "745feb5ea96cb6ff39a76b2821c57591fd70b528325562486d47b5d08900e2e4",
        url = "https://github.com/bazelbuild/buildtools/releases/download/5.1.0/buildifier-darwin-arm64",
    ),
)

http_file(
    name = "buildifier_macos_amd64",
    downloaded_file_path = "buildifier",
    executable = True,
    sha256 = "c9378d9f4293fc38ec54a08fbc74e7a9d28914dae6891334401e59f38f6e65dc",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "c9378d9f4293fc38ec54a08fbc74e7a9d28914dae6891334401e59f38f6e65dc",
        url = "https://github.com/bazelbuild/buildtools/releases/download/5.1.0/buildifier-darwin-amd64",
    ),
)

###########
# protoc. #
###########

# The following archives were taken from
# https://github.com/protocolbuffers/protobuf/releases/tag/v21.12.
PROTOC_BUILD_FILE_CONTENT = """
exports_files(["bin/protoc"], visibility = ["//visibility:public"])
"""

http_archive(
    name = "protoc_linux_x64",
    build_file_content = PROTOC_BUILD_FILE_CONTENT,
    sha256 = "3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8",
    urls = gcs_mirror_url(
        sha256 = "3a4c1e5f2516c639d3079b1586e703fc7bcfa2136d58bda24d1d54f949c315e8",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-linux-x86_64.zip",
    ),
)

http_archive(
    name = "protoc_mac_x64",
    build_file_content = PROTOC_BUILD_FILE_CONTENT,
    sha256 = "9448ff40278504a7ae5139bb70c962acc78c32d8fc54b4890a55c14c68b9d10a",
    urls = gcs_mirror_url(
        sha256 = "9448ff40278504a7ae5139bb70c962acc78c32d8fc54b4890a55c14c68b9d10a",
        url = "https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-osx-x86_64.zip",
    ),
)
