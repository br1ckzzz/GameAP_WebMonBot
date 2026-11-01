import socket

def query_a2s(port):
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.settimeout(2.0)
    try:
        req = b'\xff\xff\xff\xffTSource Engine Query\x00'
        s.sendto(req, ('46.138.244.41', port))
        resp, _ = s.recvfrom(2048)
        if len(resp) >= 9 and resp[4] == 0x41:
            challenge = resp[5:9]
            s.sendto(req + challenge, ('46.138.244.41', port))
            resp, _ = s.recvfrom(2048)
        
        data = resp[6:] # skip header (4), type (1), proto (1)
        name = data.split(b'\x00')[0].decode('utf-8', errors='ignore')
        rest = data[len(name)+1:]
        map_name = rest.split(b'\x00')[0].decode('utf-8', errors='ignore')
        print(f"Port {port} -> Name: '{name}' | Map: '{map_name}'")
    except Exception as e:
        print(f"Port {port} error: {e}")
    finally:
        s.close()

query_a2s(16261)
query_a2s(16666)
