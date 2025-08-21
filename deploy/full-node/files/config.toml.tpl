[identity]
key_file = '/etc/piri/service.pem'

[pdp]
contract_address = '${pdp_contract_address}'
lotus_endpoint = '${pdp_lotus_endpoint}'
owner_address = '${pdp_owner_address}'

[repo]
data_dir = '/data/piri'
temp_dir = '/tmp/piri'

[server]
host = 'localhost'
port = '3000'
public_url = '${server_public_url}'

[ucan]
proof_set = '${proof_set}'

[ucan.services]
[ucan.services.indexer]
did = 'did:web:staging.indexer.warm.storacha.network'
proof = '${indexer_proof}'
url = 'https://staging.indexer.warm.storacha.network/claims'

[ucan.services.principal_mapping]
'did:web:staging.indexer.warm.storacha.network' = 'did:key:z6Mkr4QkdinnXQmJ9JdnzwhcEjR8nMnuVPEwREyh9jp2Pb7k'
'did:web:staging.up.warm.storacha.network' = 'did:key:z6MkpR58oZpK7L3cdZZciKT25ynGro7RZm6boFouWQ7AzF7v'

[ucan.services.publisher]
ipni_announce_urls = ['https://cid.contact/announce', 'https://staging.ipni.warm.storacha.network']

[ucan.services.upload]
did = 'did:web:staging.up.warm.storacha.network'
url = 'https://staging.up.warm.storacha.network'