
func maxArray(array) {
    if (len(array) == 0) {
        return 0;
    }
    var max = array[0];
    var len = len(array);
    for (var idx = 0; idx < len; idx++) {
        if (array[idx] > max) {
            max = array[idx];
        }
    }
    return max;
}

// print("MAX", max(k));

assert(false, "this needs to be fixed")
