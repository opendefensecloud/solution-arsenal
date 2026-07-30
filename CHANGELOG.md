# Changelog

## [0.3.0](https://github.com/opendefensecloud/solution-arsenal/compare/v0.3.0-rc2...v0.3.0) (2026-07-30)


### ci

* gate release-please app token on both app id and private key ([2c5d19e](https://github.com/opendefensecloud/solution-arsenal/commit/2c5d19ef738eacbce168d5f392d8d7d472f0f8fc))


### Features

* added status visualisation for the deployment workflow in the FE ([#664](https://github.com/opendefensecloud/solution-arsenal/issues/664)) ([88dc1ee](https://github.com/opendefensecloud/solution-arsenal/commit/88dc1ee1a8a6b21c6270b4b0d9381d5a64ed4d40))
* **api:** add ObjectReference type for cross-namespace resource references ([30cecba](https://github.com/opendefensecloud/solution-arsenal/commit/30cecba762ef2650fe3b3fffb987b01d36ca7da0))
* **ci:** replace release-drafter with release-please ([#632](https://github.com/opendefensecloud/solution-arsenal/issues/632)) ([da5f12f](https://github.com/opendefensecloud/solution-arsenal/commit/da5f12f14313f434962aa01d39bcdbc1674d1d55))
* **ci:** replace release-drafter with release-please ([#632](https://github.com/opendefensecloud/solution-arsenal/issues/632)) ([#676](https://github.com/opendefensecloud/solution-arsenal/issues/676)) ([64a05d9](https://github.com/opendefensecloud/solution-arsenal/commit/64a05d97cff85aaef8a0fed45b0839c905571a0e))
* crud for k8s ressources in web ui ([94e5de1](https://github.com/opendefensecloud/solution-arsenal/commit/94e5de199d58b590396ab0346bea684eadfe63e3))
* crud for k8s ressources in web ui ([#708](https://github.com/opendefensecloud/solution-arsenal/issues/708)) ([83ba01b](https://github.com/opendefensecloud/solution-arsenal/commit/83ba01b2983bb832a85c2c2da36b1bac362616ae))
* impl fe list views ([#645](https://github.com/opendefensecloud/solution-arsenal/issues/645)) ([78c7fd7](https://github.com/opendefensecloud/solution-arsenal/commit/78c7fd70edcde387d6c2a46d67569888ebaddae1))


### Bug Fixes

* **ci:** address review findings on release-please migration ([a6dec29](https://github.com/opendefensecloud/solution-arsenal/commit/a6dec297878119adb86b4e730654922d4c7f6bf0))
* **deps:** migrate cenkalti/backoff imports from v5 to v7 ([7b9ecdc](https://github.com/opendefensecloud/solution-arsenal/commit/7b9ecdcf9fb3909928ea001e7ac3c39cdc524592))
* **deps:** resolve new CVEs flagged by osv-scanner ([172c57c](https://github.com/opendefensecloud/solution-arsenal/commit/172c57c6f8397fcf7b278408dbec849d095d63a8))
* **deps:** resolve new CVEs flagged by osv-scanner ([#675](https://github.com/opendefensecloud/solution-arsenal/issues/675)) ([c7334a5](https://github.com/opendefensecloud/solution-arsenal/commit/c7334a51da5d12d7bd62157a29726e8d2760b9e6))
* **deps:** update dependencies (patch & digest) ([f94fa0f](https://github.com/opendefensecloud/solution-arsenal/commit/f94fa0f7df77a0d10beedc6e2dd9ba48b9fcb33c))
* **deps:** update dependencies (patch & digest) ([#663](https://github.com/opendefensecloud/solution-arsenal/issues/663)) ([f4339d7](https://github.com/opendefensecloud/solution-arsenal/commit/f4339d72441583a3e0624fbafac0e20e05c0b25e))
* **deps:** update dependencies (patch & digest) ([#698](https://github.com/opendefensecloud/solution-arsenal/issues/698)) ([29fc135](https://github.com/opendefensecloud/solution-arsenal/commit/29fc13539ef45fc386183b145e21a01f9e21dec5))
* **deps:** update go version in remaining places ([748b48f](https://github.com/opendefensecloud/solution-arsenal/commit/748b48ff79c6931f06c63aa1e80383f605552310))
* **deps:** update go-overlay to provide go 1.26.5 ([e10b8af](https://github.com/opendefensecloud/solution-arsenal/commit/e10b8af60767aab0a01293886e34174b4c4f3007))
* **deps:** update kubernetes dependencies ([7037699](https://github.com/opendefensecloud/solution-arsenal/commit/7037699e2cad66021920db5e6c97c5c769768625))
* **deps:** update kubernetes dependencies ([#682](https://github.com/opendefensecloud/solution-arsenal/issues/682)) ([aefbd19](https://github.com/opendefensecloud/solution-arsenal/commit/aefbd194e1f7b0221325458206f42337dfd6a133))
* **deps:** update module github.com/cenkalti/backoff/v5 to v7 ([96592ba](https://github.com/opendefensecloud/solution-arsenal/commit/96592bad59b74023106305ca0c934a712d898ac7))
* **deps:** update module github.com/cenkalti/backoff/v5 to v7 ([5ae5262](https://github.com/opendefensecloud/solution-arsenal/commit/5ae52624a8d8c4d68f8cc14d984099ee0433d984))
* **deps:** update module github.com/cenkalti/backoff/v5 to v7 ([fb845f5](https://github.com/opendefensecloud/solution-arsenal/commit/fb845f59d7321224a989abb98b650a69a7ba323f))
* **deps:** update module github.com/cenkalti/backoff/v5 to v7 ([#671](https://github.com/opendefensecloud/solution-arsenal/issues/671)) ([6518ca3](https://github.com/opendefensecloud/solution-arsenal/commit/6518ca3955deaba708f16e8325304f58a753e22f))
* **deps:** update module github.com/cenkalti/backoff/v5 to v7 ([#674](https://github.com/opendefensecloud/solution-arsenal/issues/674)) ([09777c6](https://github.com/opendefensecloud/solution-arsenal/commit/09777c63b221c9cbc2adaa1f3646be8219f919ad))
* **deps:** update module github.com/cenkalti/backoff/v5 to v7 ([#716](https://github.com/opendefensecloud/solution-arsenal/issues/716)) ([6d464e2](https://github.com/opendefensecloud/solution-arsenal/commit/6d464e243e24f349c17a3485d4a375f6b3b34bc4))
* **deps:** update module oras.land/oras-go/v2 to v2.6.2 [security] ([fbe55ec](https://github.com/opendefensecloud/solution-arsenal/commit/fbe55ec2265d1013784b7615a6f6f3e0cf13626c))
* **deps:** update module oras.land/oras-go/v2 to v2.6.2 [security] ([#696](https://github.com/opendefensecloud/solution-arsenal/issues/696)) ([6b52133](https://github.com/opendefensecloud/solution-arsenal/commit/6b521336c6aed4c0b998f0ed9265e9aef4e0e460))
* **discovery:** use errors.Is for http.ErrServerClosed check ([7887109](https://github.com/opendefensecloud/solution-arsenal/commit/7887109e215788a151508b511f11adccd29b6d44))
* **discovery:** use errors.Is for http.ErrServerClosed check ([#693](https://github.com/opendefensecloud/solution-arsenal/issues/693)) ([be17b8f](https://github.com/opendefensecloud/solution-arsenal/commit/be17b8ffbaf548860936ad6a28cbf580ac83f352))
* **discovery:** use strings.Cut to strip digest algorithm prefix ([5727e25](https://github.com/opendefensecloud/solution-arsenal/commit/5727e25e70bb36c811004b00a2d09372cb782381))
* **discovery:** use strings.Cut to strip digest algorithm prefix ([#694](https://github.com/opendefensecloud/solution-arsenal/issues/694)) ([1bf1856](https://github.com/opendefensecloud/solution-arsenal/commit/1bf1856e1a25858c6b21a191ea14689e7ca771be))
* document reference ([629555a](https://github.com/opendefensecloud/solution-arsenal/commit/629555aec5ed821ebad448096367e51bcf5c7670))
* handle insecure deploy registry for bootstrap chart ([#660](https://github.com/opendefensecloud/solution-arsenal/issues/660)) ([fb8de30](https://github.com/opendefensecloud/solution-arsenal/commit/fb8de30ed7fbd2e20b2ce7357909697dc5abad4d))
* registry resources ([f42dd17](https://github.com/opendefensecloud/solution-arsenal/commit/f42dd170f2c9305ec6b5d67ae6136f20c6de4c72))
* registry resources ([#700](https://github.com/opendefensecloud/solution-arsenal/issues/700)) ([756528e](https://github.com/opendefensecloud/solution-arsenal/commit/756528edc5d3ad431da56a1996a5171cb3547e6f))
* remove orphaned devenv.nix file ([935c4ed](https://github.com/opendefensecloud/solution-arsenal/commit/935c4ed2e6e04b319d9fff3c9c29e3cd329f0c2f))
* remove orphaned devenv.nix file ([#705](https://github.com/opendefensecloud/solution-arsenal/issues/705)) ([7dc25ca](https://github.com/opendefensecloud/solution-arsenal/commit/7dc25ca7f202e7dd2594c4a08cd3eff8fd3eeb0a))
* use local dir for dex certs ([#638](https://github.com/opendefensecloud/solution-arsenal/issues/638)) ([8f875c8](https://github.com/opendefensecloud/solution-arsenal/commit/8f875c8edb0eac04b502b6450699aa6a4799430a))


### Miscellaneous Chores

* add missing entries to the path filter of GitHub actions ([d77416e](https://github.com/opendefensecloud/solution-arsenal/commit/d77416e48270abe61419db0e2fe3913ee16c84e8))
* add missing entries to the path filter of GitHub actions ([#670](https://github.com/opendefensecloud/solution-arsenal/issues/670)) ([101e4c0](https://github.com/opendefensecloud/solution-arsenal/commit/101e4c02d1d4a8b6c8e75b1f9bcd8adac487e2d2))
* **deps:** update actions/cache action to v6 ([cdd49e2](https://github.com/opendefensecloud/solution-arsenal/commit/cdd49e219184a935ff2df723df1916b76c3a2fbe))
* **deps:** update actions/cache action to v6 ([#692](https://github.com/opendefensecloud/solution-arsenal/issues/692)) ([23dd707](https://github.com/opendefensecloud/solution-arsenal/commit/23dd7070f87db0f36afc9cdb2b5a7456016918b3))
* **deps:** update dependencies (patch & digest) ([5a7fe48](https://github.com/opendefensecloud/solution-arsenal/commit/5a7fe484db987308526950a8bf453126cfbe2586))
* **deps:** update dependencies (patch & digest) ([a75d502](https://github.com/opendefensecloud/solution-arsenal/commit/a75d50284f693c1bd6ec0ffd85d5fdc96efe31da))
* **deps:** update dependencies (patch & digest) ([#683](https://github.com/opendefensecloud/solution-arsenal/issues/683)) ([17728f1](https://github.com/opendefensecloud/solution-arsenal/commit/17728f13cd50568af384d1f4288f1caab8ab63c6))
* **deps:** update dependency go to v1.26.5 ([2bf738b](https://github.com/opendefensecloud/solution-arsenal/commit/2bf738b010c36cfea3231297234e6a0c7c70ec85))
* **deps:** update dependency pillow to v12.3.0 [security] ([1434bd4](https://github.com/opendefensecloud/solution-arsenal/commit/1434bd48fc11049310de2234f0da92198ccae6c2))
* **deps:** update dependency pillow to v12.3.0 [security] ([#699](https://github.com/opendefensecloud/solution-arsenal/issues/699)) ([d658e68](https://github.com/opendefensecloud/solution-arsenal/commit/d658e68c0c61810b12d6fcde75d40cba417a86a4))
* **deps:** update dependency typescript to v7 ([8c81e3a](https://github.com/opendefensecloud/solution-arsenal/commit/8c81e3aa5c09499935f3ca1ab20184b107991acf))
* **deps:** update dependency typescript to v7 ([#691](https://github.com/opendefensecloud/solution-arsenal/issues/691)) ([349cd5f](https://github.com/opendefensecloud/solution-arsenal/commit/349cd5f5737a1a5f3a59fcf190ef9e22d1a384a6))
* **deps:** update golang version sync to v1.26.5 ([#697](https://github.com/opendefensecloud/solution-arsenal/issues/697)) ([422b1d4](https://github.com/opendefensecloud/solution-arsenal/commit/422b1d45f027ce44ac221a7ff53788501d976118))
* **deps:** update golang:1.26.4 docker digest to 32c0e6e ([#662](https://github.com/opendefensecloud/solution-arsenal/issues/662)) ([2833f64](https://github.com/opendefensecloud/solution-arsenal/commit/2833f64a691126ed72fc1b3f4b52d7abf97c22d6))
* **deps:** update golang:1.26.4 docker digest to f96cc55 ([e1ced04](https://github.com/opendefensecloud/solution-arsenal/commit/e1ced048d462d9c56f04e9f961d1c4921b06596e))
* **deps:** update golang:1.26.5 docker digest to 3aff665 ([a664b76](https://github.com/opendefensecloud/solution-arsenal/commit/a664b76e8a135a598ab0e4b43013b03649e3fe89))
* **deps:** update golang:1.26.5 docker digest to 3aff665 ([#712](https://github.com/opendefensecloud/solution-arsenal/issues/712)) ([afe9193](https://github.com/opendefensecloud/solution-arsenal/commit/afe91930040e23a00f0ffd2b166faad91ab438a8))
* fix target release view ([e347937](https://github.com/opendefensecloud/solution-arsenal/commit/e34793791157168dee2d8921090c4c31e6f69dad))
* improve CI docker caching ([#633](https://github.com/opendefensecloud/solution-arsenal/issues/633)) ([213f23c](https://github.com/opendefensecloud/solution-arsenal/commit/213f23c823fb26c97c18726a21f1ca05c03dda4d))
* improved accessability ([2dacb25](https://github.com/opendefensecloud/solution-arsenal/commit/2dacb254f17dce04333c31eed537b01c8f364419))
* renamed make targets ([a6a901d](https://github.com/opendefensecloud/solution-arsenal/commit/a6a901dfb2d911d818ac5f4c779da5ef0b0fb636))
* split up unit and integration tests, added tests ([ec6a9ea](https://github.com/opendefensecloud/solution-arsenal/commit/ec6a9eada1cff1e8fedfe3cb860d4260276927e9))
* split up unit and integration tests, added tests ([#684](https://github.com/opendefensecloud/solution-arsenal/issues/684)) ([1aca0e4](https://github.com/opendefensecloud/solution-arsenal/commit/1aca0e45da6b31e6ae9cb9984410e0f41b1804f6))
* trust-scope docker layer cache, tidy Dockerfile mounts ([17dac28](https://github.com/opendefensecloud/solution-arsenal/commit/17dac28eaef5fe5cdee746f702f78453defee2f7))
