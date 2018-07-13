get-python:
	kubeless function deploy get-python --runtime python2.7 --handler helloget.foo --from-file python/helloget.py

get-python-verify:
	kubeless function call get-python |egrep hello.world
	kubeless function top --function get-python --out yaml |egrep total_calls.*[1-100000]

