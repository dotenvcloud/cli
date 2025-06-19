// Node.js script to test encryption compatibility with Go implementation

const crypto = require('crypto');

function testEncryption(key, plaintext) {
    const iv = crypto.randomBytes(12); // 12 bytes for GCM
    const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
    
    // Encrypt
    let encrypted = cipher.update(plaintext, 'utf8');
    encrypted = Buffer.concat([encrypted, cipher.final()]);
    const tag = cipher.getAuthTag();
    
    // Format: base64(IV || ciphertext || tag)
    const result = Buffer.concat([iv, encrypted, tag]);
    
    return {
        plaintext: plaintext,
        key_base64: key.toString('base64'),
        iv_base64: iv.toString('base64'),
        ciphertext_base64: result.toString('base64'),
        tag_base64: tag.toString('base64')
    };
}

function testDecryption(ciphertext_base64, key) {
    // Decode from base64
    const data = Buffer.from(ciphertext_base64, 'base64');
    
    // Extract components
    const iv = data.slice(0, 12);
    const tag = data.slice(-16);
    const ciphertext = data.slice(12, -16);
    
    // Decrypt
    const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);
    decipher.setAuthTag(tag);
    
    let decrypted = decipher.update(ciphertext);
    decrypted = Buffer.concat([decrypted, decipher.final()]);
    
    return decrypted.toString('utf8');
}

// Test vectors
const test_key = Buffer.alloc(32); // 32 zeros for testing
const test_cases = [
    "Hello, World!",
    "",
    "Unicode: 你好世界 🌍",
    "Special chars: !@#$%^&*()"
];

console.log("Node.js Encryption Test Results");
console.log("===============================\n");

test_cases.forEach(plaintext => {
    try {
        const result = testEncryption(test_key, plaintext);
        console.log("Plaintext:", JSON.stringify(plaintext));
        console.log("Ciphertext:", result.ciphertext_base64);
        
        // Test decryption
        const decrypted = testDecryption(result.ciphertext_base64, test_key);
        console.log("Decrypted:", JSON.stringify(decrypted));
        console.log("Match:", decrypted === plaintext ? "✓" : "✗");
        console.log();
    } catch (error) {
        console.error("Error:", error.message);
        console.log();
    }
});

// Test with Go-generated ciphertext (if provided as command line argument)
if (process.argv[2]) {
    console.log("Testing Go-generated ciphertext");
    console.log("================================");
    const go_ciphertext = process.argv[2];
    
    try {
        const decrypted = testDecryption(go_ciphertext, test_key);
        console.log("Go ciphertext:", go_ciphertext);
        console.log("Decrypted:", JSON.stringify(decrypted));
    } catch (error) {
        console.error("Error:", error.message);
    }
}

// Generate known test vectors with fixed IV for compatibility testing
console.log("\nFixed IV Test Vectors");
console.log("====================");

const fixed_iv = Buffer.alloc(12); // 12 zeros
const fixed_key = Buffer.alloc(32); // 32 zeros

test_cases.forEach(plaintext => {
    const cipher = crypto.createCipheriv('aes-256-gcm', fixed_key, fixed_iv);
    
    let encrypted = cipher.update(plaintext, 'utf8');
    encrypted = Buffer.concat([encrypted, cipher.final()]);
    const tag = cipher.getAuthTag();
    
    const result = Buffer.concat([fixed_iv, encrypted, tag]);
    
    console.log(`Plaintext: ${JSON.stringify(plaintext)}`);
    console.log(`Key: ${fixed_key.toString('base64')}`);
    console.log(`IV: ${fixed_iv.toString('base64')}`);
    console.log(`Ciphertext: ${result.toString('base64')}`);
    console.log();
});