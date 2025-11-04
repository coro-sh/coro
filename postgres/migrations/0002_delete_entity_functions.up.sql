CREATE OR REPLACE FUNCTION delete_namespace_and_nkeys(ns_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete user nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM "user" u WHERE u.namespace_id = ns_id);

    -- delete account nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM account a WHERE a.namespace_id = ns_id);

    -- delete operator nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM operator o WHERE o.namespace_id = ns_id);

    -- delete the namespace (cascading operators, accounts, and users)
    DELETE FROM namespace WHERE id = ns_id;

END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_operator_and_nkeys(op_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete user nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM "user" u WHERE u.operator_id = op_id);

    -- delete account nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM account a WHERE a.operator_id = op_id);

    -- delete the operator nkey
    DELETE FROM nkey WHERE id = op_id;

    -- delete the operator (cascading accounts and users)
    DELETE FROM operator WHERE id = op_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_account_and_nkeys(acc_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete user nkeys
    DELETE
    FROM nkey
    WHERE id IN (SELECT id FROM "user" u WHERE u.account_id = acc_id);

    -- delete the account nkey
    DELETE FROM nkey WHERE id = acc_id;

    -- delete the account (cascading users)
    DELETE FROM account WHERE id = acc_id;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_user_and_nkey(user_id TEXT)
    RETURNS VOID AS
$$
BEGIN
    -- delete the user nkey
    DELETE FROM nkey WHERE id = user_id;

    -- delete the user
    DELETE FROM "user" WHERE id = user_id;
END;
$$ LANGUAGE plpgsql;