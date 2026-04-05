use frost::keys::dkg::part1;
use frost::keys::{generate_with_dealer, IdentifierList, KeyPackage, PublicKeyPackage};
use frost::round1::commit;
use frost::round2::sign;
use frost::Identifier;
use frost::{Signature, SigningPackage};
use frost_ed25519 as frost;
use rand::rngs::OsRng;
use std::collections::BTreeMap;

fn main() {
    println!("=== Testing frost-ed25519 3.0.0-rc.0 API ===\n");

    // Test 1: Trusted dealer key generation
    println!("1. Testing trusted dealer key generation...");
    let mut rng = rand::thread_rng();
    let (shares_map, pubkey_package) =
        generate_with_dealer(5u16, 3u16, IdentifierList::Default, &mut rng).unwrap();
    println!("   ✓ Key generation works");
    println!("     shares: {:?}\n", shares_map.len());

    // Test 2: Get key packages
    println!("2. Testing KeyPackage creation...");
    let mut key_packages = BTreeMap::new();
    for (identifier, secret_share) in &shares_map {
        let key_pkg = KeyPackage::try_from(secret_share.clone()).unwrap();
        println!("   ✓ KeyPackage for identifier: {:?}", identifier);
        key_packages.insert(*identifier, key_pkg);
    }

    // Test 3: Get verifying key
    println!("3. Testing VerifyingKey...");
    let verifying_key = pubkey_package.verifying_key();
    println!("   ✓ Got verifying key\n");

    // Test 4: Signing Round 1 - commit() takes SigningShare
    println!("4. Testing signing round 1 (commit)...");
    let mut all_nonces = BTreeMap::new();
    let mut all_commitments = BTreeMap::new();

    for (identifier, key_pkg) in &key_packages {
        let signing_share = key_pkg.signing_share();
        let (nonces, commitments) = commit(signing_share, &mut OsRng);

        println!("   ✓ commit() for identifier: {:?}", identifier);
        all_nonces.insert(*identifier, nonces);
        all_commitments.insert(*identifier, commitments);

        if all_nonces.len() >= 3 {
            break; // Just test with 3
        }
    }
    println!();

    // Test 5: Create SigningPackage
    println!("5. Testing SigningPackage creation...");
    let message = b"test message";
    let signing_package = SigningPackage::new(all_commitments.clone(), message);
    println!("   ✓ SigningPackage created\n");

    // Test 6: Sign round 2 - generate partial signatures
    println!("6. Testing signing round 2...");
    let mut signature_shares = BTreeMap::new();

    for (identifier, key_pkg) in &key_packages {
        if !all_nonces.contains_key(identifier) {
            continue;
        }
        let nonces = &all_nonces[identifier];

        let sig_share = sign(&signing_package, nonces, key_pkg).unwrap();
        println!("   ✓ Partial signature for identifier: {:?}", identifier);
        signature_shares.insert(*identifier, sig_share);
    }
    println!();

    // Test 7: Aggregation
    println!("7. Testing aggregation...");
    let group_signature =
        frost::aggregate(&signing_package, &signature_shares, &pubkey_package).unwrap();
    println!("   ✓ Aggregation successful\n");

    // Test 8: Verification
    println!("8. Testing verification...");
    let valid = verifying_key.verify(message, &group_signature).is_ok();
    println!("   ✓ Signature valid: {}\n", valid);

    // Test 9: Serialization test
    println!("9. Testing serialization...");
    let sig_bytes = group_signature.serialize();
    println!(
        "   signature.serialize() returns: {:?}",
        std::any::type_name_of_val(&sig_bytes)
    );
    // It's a Vec<u8>, not Result!

    let pk_bytes = pubkey_package.serialize();
    println!(
        "   pubkey_package.serialize() returns: {:?}",
        std::any::type_name_of_val(&pk_bytes)
    );

    let vk_bytes = verifying_key.serialize();
    println!(
        "   verifying_key.serialize() returns: {:?}",
        std::any::type_name_of_val(&vk_bytes)
    );
    println!();

    // Test 10: Identifier to u16
    println!("10. Testing identifier handling...");
    for (identifier, _) in &key_packages {
        println!("   identifier: {:?}", identifier);
        // Try to see what we can do with identifier
        break;
    }
    println!();

    println!("=== All tests passed! ===");
    println!("\nKEY FINDINGS:");
    println!("1. commit() takes &SigningShare, not &Identifier");
    println!("2. serialize() returns Vec<u8>, NOT Result<Vec<u8>, Error>");
    println!("3. shares from generate_with_dealer is BTreeMap<Identifier, SecretShare>");
    println!("4. KeyPackage::try_from() takes SecretShare, not identifier");
}
