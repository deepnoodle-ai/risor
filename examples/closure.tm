#!/usr/bin/env risor

var testValue = 100

func getint() {
    var foo = testValue + 1
    func inner() {
        foo
    }
    return inner
}

print(getint()())
