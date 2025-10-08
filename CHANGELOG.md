# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.2.0] - 2024-10-08

### Added

- MultiTransfer with callback support functionality
- Enhanced PostWithFunc for better error handling
- Empty pack validation for MultiTransferWaitCallback
- Improved callback handling mechanisms
- Enhanced error handling and documentation for transfer operations

### Changed

- Enhanced MultiTransferWaitCallback with improved validation
- Updated examples in PkgMain and PkgQueue for better usage patterns
- Improved queue HTTP functionality

### Deprecated

- MultiTransfer function - now deprecated, recommend using MultiTransferWaitCallback instead

### Removed

- Obsolete test files for transfer functionality:
  - pkg/transfer/errors_test.go
  - pkg/transfer/mock_http.go
  - pkg/transfer/model_test.go
  - pkg/transfer/multi_transfer_test.go
  - pkg/transfer/retry_recover_test.go
  - pkg/transfer/single_transfer_test.go
  - pkg/transfer/transfer_test.go
- Removed pkg/bko-bankpkg.go file

### Commits

- `ce048be` - feat(transfer): mark MultiTransfer as deprecated and recommend MultiTransferWaitCallback
- `c299375` - feat(transfer): enhance error handling and documentation for transfer operations
- `25e985a` - refactor(transfer): remove obsolete test files for transfer functionality
- `2fe3ab2` - feat(transfer): enhance MultiTransferWaitCallback with empty pack validation and improved callback handling
- `9ae89d0` - feat(transfer): implement MultiTransfer with callback support and enhance PostWithFunc for better error handling

## [v0.1.0] - Previous Release

Initial release baseline.
