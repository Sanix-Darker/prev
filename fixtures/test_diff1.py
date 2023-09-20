def get_bigger_value(given_dict: dict) -> int:

    maxx = 0
    for k in given_dict.keys():
        if given_dict[k] > maxx:
            maxx = given_dict[k]

    return maxx

# this is a comment
# this is a comment

class Test:
    val = 1
