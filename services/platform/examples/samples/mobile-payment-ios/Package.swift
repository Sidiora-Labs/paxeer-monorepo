// swift-tools-version: 5.9
// SPDX-License-Identifier: Apache-2.0
import PackageDescription

let package = Package(
    name: "LayerXMobilePayment",
    platforms: [.iOS(.v15), .macOS(.v12)],
    dependencies: [
        .package(path: "../../../../../sdk/integrations/ios"),
        .package(path: "../../../../../sdk/platform/swift")
    ],
    targets: [
        .executableTarget(
            name: "MobilePayment",
            dependencies: [
                .product(name: "LayerXMobile", package: "ios"),
                .product(name: "LayerXSDK", package: "swift")
            ],
            path: "Sources/MobilePayment"
        )
    ]
)
