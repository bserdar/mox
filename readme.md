# Mox: Mocks service calls

Describe service calls and what to return using a JSON document. You
can pass a single item, or an array:

```
[
{
    "method":"GET",
    "path":"/mysvc/call",
    "queries": [
       {"key":"queryKey1","value":"queryValue1"}, 
       {"key":"queryKey2","value":"queryValue2"}, 
    ],
    "return":{
        "status":200,
        "headers": [
            {"key":"Content-Type","value":"application/json"}
        ],
        "body":"{\"field\":\"value\"}"
    }
}
]
```
You need to escape return.body properly so it is valid JSON.

HTTP POST this to the admin port (8001). Then, you can call your API at port 8000:

```
  curl "http://localhost:8000/mysvc/call?queryKey1=queryValue1&queryKey2=queryValue2"
  {"field":"value"}
```
