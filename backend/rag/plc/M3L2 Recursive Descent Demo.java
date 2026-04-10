package plc.project.parser;

import com.google.common.base.Preconditions;
import plc.project.lexer.Token;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * This style of parser is called <em>recursive descent</em>. Each rule in our
 * grammar has dedicated function, and references to other rules correspond to
 * calling that function. Recursive rules are therefore supported by actual
 * recursive calls, while operator precedence is encoded via the grammar.
 *
 * <p>The parser has a similar architecture to the lexer, just with
 * {@link Token}s instead of characters. As before, {@link TokenStream#peek} and
 * {@link TokenStream#match} help with traversing the token stream. Instead of
 * emitting tokens, you will instead need to extract the literal value via
 * {@link TokenStream#get} to be added to the relevant AST.
 */
public final class Parser {

    private final TokenStream tokens;

    public Parser(List<Token> tokens) {
        this.tokens = new TokenStream(tokens);
    }

    public Ast parse(String rule) throws ParseException {
        var ast = switch (rule) {
            case "source" -> parseSource();
            case "stmt" -> parseStmt();
            case "expr" -> parseExpr();
            default -> throw new AssertionError(rule);
        };
        if (tokens.has(0)) {
            throw new ParseException("Expected end of input.", tokens.getNext());
        }
        return ast;
    }

    private Ast.Source parseSource() throws ParseException {
        var statements = new ArrayList<Ast.Stmt>();
        while (tokens.has(0)) {
            statements.add(parseStmt());
        }
        return new Ast.Source(statements);
    }

    private Ast.Stmt parseStmt() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Stmt parseLetStmt() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Stmt parseDefStmt() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Stmt parseIfStmt() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Stmt parseForStmt() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Stmt parseReturnStmt() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Stmt parseExpressionOrAssignmentStmt() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parseExpr() throws ParseException {
        return parseAdditiveExpr(); //TODO
    }

    private Ast.Expr parseLogicalExpr() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parseComparisonExpr() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parseAdditiveExpr() throws ParseException {
        //additive ::= mult_expr (('+' | '-') mult_expr)*
        var expr = parseMultiplicativeExpr();
        while (tokens.match("+") || tokens.match("-")) {
            var operator = tokens.get(-1).literal();
            var right = parseMultiplicativeExpr();
            expr = new Ast.Expr.Binary(operator, expr, right);
        }
        return expr;
    }

    private Ast.Expr parseMultiplicativeExpr() throws ParseException {
        //additive ::= primary (('+' | '-') primary)*
        //add 1 + 2
        var expr = parsePrimaryExpr();
        while (tokens.match("*") || tokens.match("/")) {
            var operator = tokens.get(-1).literal();
            var right = parsePrimaryExpr();
            expr = new Ast.Expr.Binary(operator, expr, right);
        }
        return expr;
    }

    private Ast.Expr parseSecondaryExpr() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parsePropertyOrMethod(Ast.Expr receiver) throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parsePrimaryExpr() throws ParseException {
        if (tokens.match(Token.Type.INTEGER)) {
            var literal = tokens.get(-1).literal();
            return new Ast.Expr.Literal(new BigInteger(literal));
        } else if (tokens.peek(Token.Type.IDENTIFIER)) {
            return parseVariableOrFunctionExpr();
        }
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parseGroupExpr() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parseObjectExpr() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private Ast.Expr parseVariableOrFunctionExpr() throws ParseException {
        throw new UnsupportedOperationException("TODO"); //TODO
    }

    private static final class TokenStream {

        private final List<Token> tokens;
        private int index = 0;

        private TokenStream(List<Token> tokens) {
            this.tokens = tokens;
        }

        /**
         * Returns true if there is a token at (index + offset).
         */
        public boolean has(int offset) {
            return index + offset < tokens.size();
        }

        /**
         * Returns the token at (index + offset).
         */
        public Token get(int offset) {
            Preconditions.checkState(has(offset));
            return tokens.get(index + offset);
        }

        /**
         * Returns the next token, if present.
         */
        public Optional<Token> getNext() {
            return index < tokens.size() ? Optional.of(tokens.get(index)) : Optional.empty();
        }

        /**
         * Returns true if the next characters match their corresponding
         * pattern. Each pattern is either a {@link Token.Type}, matching tokens
         * of that type, or a {@link String}, matching tokens with that literal.
         * In effect, {@code new Token(Token.Type.IDENTIFIER, "literal")} is
         * matched by both {@code peek(Token.Type.IDENTIFIER)} and
         * {@code peek("literal")}.
         */
        public boolean peek(Object... patterns) {
            if (!has(patterns.length - 1)) {
                return false;
            }
            for (int offset = 0; offset < patterns.length; offset++) {
                var token = tokens.get(index + offset);
                var pattern = patterns[offset];
                Preconditions.checkState(pattern instanceof Token.Type || pattern instanceof String, pattern);
                if (!token.type().equals(pattern) && !token.literal().equals(pattern)) {
                    return false;
                }
            }
            return true;
        }

        /**
         * Equivalent to peek, but also advances the token stream.
         */
        public boolean match(Object... patterns) {
            var peek = peek(patterns);
            if (peek) {
                index += patterns.length;
            }
            return peek;
        }

    }

}
