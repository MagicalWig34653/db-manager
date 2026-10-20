// SPDX-License-Identifier: AGPL-3.0-only
// Compile-only probe: no credential is created and no authorization is performed.
import AuthenticationServices
import Foundation

@available(macOS 15.0, iOS 18.0, *)
func configurePlatformPRF(
    registration: ASAuthorizationPlatformPublicKeyCredentialRegistrationRequest,
    assertion: ASAuthorizationPlatformPublicKeyCredentialAssertionRequest,
    salt: Data
) {
    registration.prf = .checkForSupport
    assertion.prf = .inputValues(.init(saltInput1: salt))
}

@available(macOS 26.4, iOS 26.4, *)
func configureSecurityKeyPRF(
    registration: ASAuthorizationSecurityKeyPublicKeyCredentialRegistrationRequest,
    salt: Data
) {
    registration.prf = .inputValues(.init(saltInput1: salt))
}
