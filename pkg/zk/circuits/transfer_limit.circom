pragma circom 2.1.8;

template TransferLimit() {
    signal input daily_limit;
    signal input spent_today;
    signal input transfer_amount;
    signal output valid;

    signal remaining;
    remaining <== daily_limit - spent_today;

    signal diff;
    diff <== remaining - transfer_amount;

    valid <== diff;
}

component main {public [transfer_amount]} = TransferLimit();