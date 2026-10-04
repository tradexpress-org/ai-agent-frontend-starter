import base64
import hashlib
import hmac
import struct
import time


def generate_totp(secret_base32: str, time_step: int = 30, digits: int = 6) -> str:
    """Generates a standard TOTP code from a Base32 secret key."""
    # Clean up key formatting (strip spaces and uppercase)
    cleaned_secret = secret_base32.replace(" ", "").upper()

    # Decode Base32 secret key
    key = base64.b32decode(cleaned_secret, casefold=True)

    # Calculate 8-byte time counter (seconds since Unix epoch / step window)
    counter = int(time.time()) // time_step
    counter_bytes = struct.pack(">Q", counter)

    # Calculate HMAC-SHA1 digest
    hmac_digest = hmac.new(key, counter_bytes, hashlib.sha1).digest()

    # Dynamic truncation to extract 4-byte code
    offset = hmac_digest[-1] & 0x0F
    code_int = (
        struct.unpack(">I", hmac_digest[offset : offset + 4])[0] & 0x7FFFFFFF
    )

    # Format as 6-digit padded string
    totp = str(code_int % (10**digits)).zfill(digits)
    return totp


# Example usage:
if __name__ == "__main__":
    # Replace with your Base32 secret key
    sample_secret = "JBSWY3DPEHPK3PXP"
    code = generate_totp(sample_secret)

    time_remaining = 30 - (int(time.time()) % 30)
    print(f"Current TOTP Code: {code}")
    print(f"Valid for next {time_remaining} seconds.")
