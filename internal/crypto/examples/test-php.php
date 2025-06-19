<?php
// PHP script to test encryption compatibility with Go implementation

function testEncryption($key, $plaintext) {
    $method = 'aes-256-gcm';
    $iv = random_bytes(12); // 12 bytes for GCM
    $tag = '';
    
    // Encrypt
    $ciphertext = openssl_encrypt(
        $plaintext,
        $method,
        $key,
        OPENSSL_RAW_DATA,
        $iv,
        $tag
    );
    
    // Format: base64(IV || ciphertext || tag)
    $result = base64_encode($iv . $ciphertext . $tag);
    
    return [
        'plaintext' => $plaintext,
        'key_base64' => base64_encode($key),
        'iv_base64' => base64_encode($iv),
        'ciphertext_base64' => $result,
        'tag_base64' => base64_encode($tag)
    ];
}

function testDecryption($ciphertext_base64, $key) {
    $method = 'aes-256-gcm';
    
    // Decode from base64
    $data = base64_decode($ciphertext_base64);
    
    // Extract components
    $iv = substr($data, 0, 12);
    $ciphertext = substr($data, 12, -16);
    $tag = substr($data, -16);
    
    // Decrypt
    $plaintext = openssl_decrypt(
        $ciphertext,
        $method,
        $key,
        OPENSSL_RAW_DATA,
        $iv,
        $tag
    );
    
    return $plaintext;
}

// Test vectors
$test_key = str_repeat("\x00", 32); // 32 zeros for testing
$test_cases = [
    "Hello, World!",
    "",
    "Unicode: 你好世界 🌍",
    "Special chars: !@#$%^&*()",
];

echo "PHP Encryption Test Results\n";
echo "===========================\n\n";

foreach ($test_cases as $plaintext) {
    $result = testEncryption($test_key, $plaintext);
    echo "Plaintext: " . json_encode($plaintext) . "\n";
    echo "Ciphertext: " . $result['ciphertext_base64'] . "\n";
    
    // Test decryption
    $decrypted = testDecryption($result['ciphertext_base64'], $test_key);
    echo "Decrypted: " . json_encode($decrypted) . "\n";
    echo "Match: " . ($decrypted === $plaintext ? "✓" : "✗") . "\n\n";
}

// Test with Go-generated ciphertext (if provided as command line argument)
if (isset($argv[1])) {
    echo "Testing Go-generated ciphertext\n";
    echo "================================\n";
    $go_ciphertext = $argv[1];
    
    try {
        $decrypted = testDecryption($go_ciphertext, $test_key);
        echo "Go ciphertext: " . $go_ciphertext . "\n";
        echo "Decrypted: " . json_encode($decrypted) . "\n";
    } catch (Exception $e) {
        echo "Error: " . $e->getMessage() . "\n";
    }
}