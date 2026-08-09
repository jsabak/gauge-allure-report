# Parameters and tables

| account_id | password        | token        | build_timestamp |
| account-1  | secret-password | secret-token | 2026-08-07T00:00:00Z |
| account-2  | other-password  | other-token  | 2026-08-08T00:00:00Z |

## Specification table row

* Observe account <account_id> password <password> token <token> build <build_timestamp>

## Scenario table row

| currency | amount |
| PLN      | 100    |
| EUR      | 25     |

* Observe currency <currency> amount <amount>

## Special parameters

* Observe static value "literal"
* Observe multiline text
    """
    first line
    second line
    """
* Observe item table
    | item | count |
    | milk | 1     |
    | tea  | 2     |

## Nested concepts

* Outer operation "zażółć gęślą"
