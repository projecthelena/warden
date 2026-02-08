import argparse
import json
import urllib.request
import urllib.error
import sys

# Usage: python create_stack.py --key <API_KEY> --group "My Service Stack" --urls http://service1.com http://service2.com

def create_request(url, data, api_key):
    req = urllib.request.Request(
        url,
        data=json.dumps(data).encode('utf-8'),
        headers={
            'Content-Type': 'application/json',
            'Authorization': f'Bearer {api_key}'
        },
        method='POST'
    )
    return req

def main():
    parser = argparse.ArgumentParser(description="Create a monitoring group and add monitors via Warden API")
    parser.add_argument("--host", default="http://localhost:9090", help="Base URL of Warden (default: http://localhost:9090)")
    parser.add_argument("--key", required=True, help="Your API Key")
    parser.add_argument("--group", required=True, help="Name of the Group to create")
    parser.add_argument("--urls", required=True, nargs='+', help="List of URLs to monitor")
    parser.add_argument("--interval", type=int, default=60, help="Check interval in seconds (default: 60)")

    args = parser.parse_args()
    base_url = args.host.rstrip('/')
    
    print(f"🔹 Target: {base_url}")
    print(f"🔹 Creating Group: '{args.group}'...")

    # 1. Create Group
    try:
        req = create_request(f"{base_url}/api/groups", {"name": args.group}, args.key)
        with urllib.request.urlopen(req) as response:
            if response.status != 201:
                print(f"❌ Failed to create group. Status: {response.status}")
                sys.exit(1)
            group_data = json.load(response)
            group_id = group_data['id']
            print(f"✅ Group Created! ID: {group_id}")
    except urllib.error.HTTPError as e:
        print(f"❌ Error creating group: {e.code} {e.reason}")
        print(e.read().decode())
        sys.exit(1)
    except Exception as e:
        print(f"❌ Error: {e}")
        sys.exit(1)

    # 2. Create Monitors
    print(f"🔹 Adding {len(args.urls)} monitors...")
    
    for url in args.urls:
        # Simple name generation from URL
        name = url.replace("https://", "").replace("http://", "").split("/")[0]
        
        payload = {
            "name": name,
            "url": url,
            "groupId": group_id,
            "interval": args.interval
        }

        try:
            req = create_request(f"{base_url}/api/monitors", payload, args.key)
            with urllib.request.urlopen(req) as response:
                if response.status == 201:
                    print(f"   ✅ Added: {url} ({name})")
                else:
                    print(f"   ⚠️  Failed to add {url}: {response.status}")
        except urllib.error.HTTPError as e:
            print(f"   ❌ Failed to add {url}: {e.code}")
        except Exception as e:
             print(f"   ❌ Error adding {url}: {e}")

    print("\n✨ Done!")

if __name__ == "__main__":
    main()
