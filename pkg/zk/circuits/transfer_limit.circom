pragma circom 2.1.8;

template TransferLimit() {
    signal private input daily_limit;
    signal private input spent_today;
    signal input transfer_amount;
    signal output valid;

    valid <== (daily_limit - spent_today - transfer_amount) * (daily_limit - spent_today - transfer_amount);
}

component main {public [transfer_amount]} = TransferLimit();